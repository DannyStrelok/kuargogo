"""
states.py - Aiogram FSM state definitions for kgg_telegram.
"""

from aiogram.fsm.state import State, StatesGroup


class BotStates(StatesGroup):
    waiting_for_confirmation = State()
    waiting_for_node = State()
    waiting_for_command = State()
    waiting_for_k3s_name = State()
    waiting_for_k3s_replicas = State()
    waiting_for_path = State()
    waiting_for_kargo_pipeline = State()
    waiting_for_kargo_freight = State()
    waiting_for_kargo_stage = State()
