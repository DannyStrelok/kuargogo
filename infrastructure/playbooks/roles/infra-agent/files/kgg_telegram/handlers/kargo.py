"""
handlers/kargo.py - Kargo GitOps pipeline promotions handler.
"""

import logging
import html
from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton
from aiogram.fsm.context import FSMContext

from kgg_telegram.helpers import run_kgg_cmd
from kgg_telegram.states import BotStates

logger = logging.getLogger(__name__)


async def handle_kargo_menu(ctx, message: Message):
    """Entry point for /kargo command or reply button.

    Lists all configured pipelines.
    """
    sent = await message.answer("⏳ <b>Fetching Kargo pipelines...</b>")
    success, out, err = await run_kgg_cmd(["kargo", "pipelines"])
    if not success:
        await sent.edit_text(f"❌ <b>Failed to fetch Kargo pipelines:</b>\n<pre>{html.escape(err)}</pre>")
        return

    pipelines = [p.strip() for p in out.strip().split("\n") if p.strip()]
    if not pipelines:
        await sent.edit_text(
            "🚢 <b>Kargo Promotions</b>\n\nNo pipelines configured under <code>gitops.pipelines</code> in your configuration."
        )
        return

    if len(pipelines) == 1:
        # Jump directly to the single pipeline dashboard
        await sent.delete()
        await _show_pipeline_dashboard(ctx, message, pipelines[0])
        return

    # Show selection menu
    kb = InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text=f"🚢 {p}", callback_data=f"kargo_pipe:{p}")] for p in pipelines
        ] + [[InlineKeyboardButton(text="🔙 Back", callback_data="cancel_action")]]
    )
    await sent.edit_text("🚢 <b>Select Kargo Pipeline:</b>", reply_markup=kb)


async def cb_kargo(ctx, query: CallbackQuery, state: FSMContext):
    """Callback router for Kargo inline button operations."""
    await query.answer()
    data = query.data.replace("kargo_", "")

    if data == "menu":
        await query.message.edit_text("⏳ <b>Fetching Kargo pipelines...</b>")
        # Back to pipeline list
        success, out, err = await run_kgg_cmd(["kargo", "pipelines"])
        if not success:
            await query.message.edit_text(f"❌ <b>Error:</b>\n<pre>{html.escape(err)}</pre>")
            return
        pipelines = [p.strip() for p in out.strip().split("\n") if p.strip()]
        kb = InlineKeyboardMarkup(
            inline_keyboard=[
                [InlineKeyboardButton(text=f"🚢 {p}", callback_data=f"kargo_pipe:{p}")] for p in pipelines
            ] + [[InlineKeyboardButton(text="🔙 Back", callback_data="cancel_action")]]
        )
        await state.clear()
        await query.message.edit_text("🚢 <b>Select Kargo Pipeline:</b>", reply_markup=kb)

    elif data.startswith("pipe:"):
        pipeline = data.split(":", 1)[1]
        await _show_pipeline_dashboard(ctx, query.message, pipeline)

    elif data.startswith("freights:"):
        pipeline = data.split(":", 1)[1]
        await query.message.edit_text("⏳ <b>Fetching available freight...</b>")
        success, out, err = await run_kgg_cmd(["kargo", "freight", "-p", pipeline])
        if not success:
            await query.message.edit_text(f"❌ <b>Failed to fetch freight:</b>\n<pre>{html.escape(err)}</pre>")
            return

        # Parse freight IDs
        lines = out.strip().split("\n")
        freight_ids = []
        for line in lines:
            trimmed = line.strip()
            # Skip header line like "📦 Available Freight in xxx:"
            if trimmed and not trimmed.startswith("📦") and not trimmed.startswith("ℹ️"):
                freight_ids.append(trimmed)

        if not freight_ids:
            kb = InlineKeyboardMarkup(
                inline_keyboard=[
                    [InlineKeyboardButton(text="🔙 Back", callback_data=f"kargo_pipe:{pipeline}")]
                ]
            )
            await query.message.edit_text(
                "ℹ️ <b>No freight available.</b>\nWait for Warehouse to discover and sync freight.",
                reply_markup=kb
            )
            return

        # Set FSM state
        await state.set_state(BotStates.waiting_for_kargo_freight)
        await state.update_data(kargo_pipeline=pipeline)

        # Build freight buttons
        # Each button shows the ID, callback is kargo_frsel:<freight_id>
        buttons = []
        for fid in freight_ids:
            # Clean ID representation for label
            label = fid
            if len(label) > 28:
                label = label[:25] + "..."
            buttons.append([InlineKeyboardButton(text=f"📦 {label}", callback_data=f"kargo_frsel:{fid}")])

        buttons.append([InlineKeyboardButton(text="🔙 Back to Dashboard", callback_data=f"kargo_pipe:{pipeline}")])
        kb = InlineKeyboardMarkup(inline_keyboard=buttons)
        await query.message.edit_text("🚀 <b>Select Freight to Promote:</b>", reply_markup=kb)

    elif data.startswith("frsel:"):
        freight_id = data.split(":", 1)[1]
        fsm_data = await state.get_data()
        pipeline = fsm_data.get("kargo_pipeline")
        if not pipeline:
            await query.message.edit_text("❌ <b>Session expired.</b> Please restart Kargo promotion.")
            await state.clear()
            return

        await query.message.edit_text("⏳ <b>Fetching target stages...</b>")
        success, out, err = await run_kgg_cmd(["kargo", "stages", "-p", pipeline])
        if not success:
            await query.message.edit_text(f"❌ <b>Failed to fetch stages:</b>\n<pre>{html.escape(err)}</pre>")
            return

        stages = [s.strip() for s in out.strip().split("\n") if s.strip()]
        if not stages:
            kb = InlineKeyboardMarkup(
                inline_keyboard=[
                    [InlineKeyboardButton(text="🔙 Back", callback_data=f"kargo_freights:{pipeline}")]
                ]
            )
            await query.message.edit_text(
                "❌ <b>No stages configured for this pipeline in kuargogo.yaml.</b>",
                reply_markup=kb
            )
            return

        await state.set_state(BotStates.waiting_for_kargo_stage)
        await state.update_data(kargo_freight=freight_id)

        # Build stage buttons
        buttons = []
        for stage in stages:
            buttons.append([InlineKeyboardButton(text=f"🏁 {stage.upper()}", callback_data=f"kargo_stgsel:{stage}")])

        buttons.append([InlineKeyboardButton(text="🔙 Back to Freights", callback_data=f"kargo_freights:{pipeline}")])
        kb = InlineKeyboardMarkup(inline_keyboard=buttons)
        await query.message.edit_text(
            f"🚀 <b>Promoting freight:</b> <code>{freight_id}</code>\n"
            f"<b>Select target Stage:</b>",
            reply_markup=kb
        )

    elif data.startswith("stgsel:"):
        stage = data.split(":", 1)[1]
        fsm_data = await state.get_data()
        pipeline = fsm_data.get("kargo_pipeline")
        freight_id = fsm_data.get("kargo_freight")

        if not pipeline or not freight_id:
            await query.message.edit_text("❌ <b>Session expired or invalid state.</b> Please restart.")
            await state.clear()
            return

        await query.message.edit_text(
            f"🚀 <b>Promoting...</b>\n\n"
            f"• Pipeline: <code>{pipeline}</code>\n"
            f"• Freight: <code>{freight_id}</code>\n"
            f"• Target Stage: <code>{stage.upper()}</code>\n\n"
            f"⏳ Executing promotion on cluster. Please wait..."
        )

        # Execute promotion command
        success, out, err = await run_kgg_cmd(["kargo", "promote", stage, freight_id, "-p", pipeline])
        if success:
            res_msg = (
                f"✅ <b>Kargo Promotion Successful!</b>\n\n"
                f"• Pipeline: <code>{pipeline}</code>\n"
                f"• Freight: <code>{freight_id}</code>\n"
                f"• Target Stage: <code>{stage.upper()}</code>\n\n"
                f"<b>CLI Output:</b>\n<pre>{html.escape(out)}</pre>"
            )
        else:
            res_msg = (
                f"❌ <b>Promotion Failed:</b>\n\n"
                f"<pre>{html.escape(err or out)}</pre>"
            )

        kb = InlineKeyboardMarkup(
            inline_keyboard=[
                [InlineKeyboardButton(text="🚢 View Pipeline Flow", callback_data=f"kargo_pipe:{pipeline}")],
                [InlineKeyboardButton(text="🚪 Close", callback_data="cancel_action")]
            ]
        )
        await query.message.edit_text(res_msg, reply_markup=kb)
        await state.clear()


async def _show_pipeline_dashboard(ctx, message: Message, pipeline: str):
    """Fetch status and render the pipeline dashboard view."""
    # We may be editing a message (callback) or replying to a command message.
    # Check if we should edit or send a new one.
    is_callback = hasattr(message, "edit_text")

    status_text = "⏳ <b>Querying Kargo live observability...</b>"
    if is_callback:
        sent = await message.edit_text(status_text)
    else:
        sent = await message.answer(status_text)

    success, out, err = await run_kgg_cmd(["kargo", "status", "-p", pipeline])
    if not success:
        kb = InlineKeyboardMarkup(
            inline_keyboard=[
                [InlineKeyboardButton(text="🔄 Retry", callback_data=f"kargo_pipe:{pipeline}")],
                [InlineKeyboardButton(text="🔙 Back", callback_data="kargo_menu")]
            ]
        )
        await sent.edit_text(
            f"❌ <b>Failed to fetch pipeline status:</b>\n<pre>{html.escape(err)}</pre>",
            reply_markup=kb
        )
        return

    # Render dashboard
    kb = InlineKeyboardMarkup(
        inline_keyboard=[
            [
                InlineKeyboardButton(text="🔄 Refresh Status", callback_data=f"kargo_pipe:{pipeline}"),
                InlineKeyboardButton(text="🚀 Promote Freight", callback_data=f"kargo_freights:{pipeline}")
            ],
            [InlineKeyboardButton(text="🔙 Back to Pipelines", callback_data="kargo_menu")]
        ]
    )
    await sent.edit_text(
        f"🚢 <b>Kargo Observability ({pipeline})</b>\n\n"
        f"<pre>{html.escape(out)}</pre>",
        reply_markup=kb
    )
