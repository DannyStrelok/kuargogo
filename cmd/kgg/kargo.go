package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/gitops"
	"github.com/spf13/cobra"
)

var kargoCmd = &cobra.Command{
	Use:   "kargo",
	Short: "Manage Kargo artifact promotions",
	Long:  `Manage Kargo promotions, warehouses, and stages directly from the CLI.`,
}

func getPipeline(cfg config.ClusterConfig, nameFlag string) (config.KargoPipeline, error) {
	if len(cfg.GitOps.Pipelines) == 0 {
		return config.KargoPipeline{}, fmt.Errorf("kargo is not configured in kuargogo.yaml")
	}

	if nameFlag != "" {
		for _, pipe := range cfg.GitOps.Pipelines {
			if pipe.Name == nameFlag {
				return pipe, nil
			}
		}
		return config.KargoPipeline{}, fmt.Errorf("pipeline %q not found in kuargogo.yaml", nameFlag)
	}

	if len(cfg.GitOps.Pipelines) == 1 {
		return cfg.GitOps.Pipelines[0], nil
	}

	var names []string
	for _, pipe := range cfg.GitOps.Pipelines {
		names = append(names, pipe.Name)
	}
	return config.KargoPipeline{}, fmt.Errorf("multiple pipelines configured (%s). Please specify which to use with --pipeline or -p", strings.Join(names, ", "))
}

var kargoPromoteCmd = &cobra.Command{
	Use:   "promote <stage> <freight>",
	Short: "Promote freight to a specific stage",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		stageName, freightID := args[0], args[1]
		cfg := config.GetConfig()
		pipelineFlag, _ := cmd.Flags().GetString("pipeline")
		p, err := getPipeline(cfg, pipelineFlag)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}

		kubeconfig, err := cfg.K3s.ExpandedKubeconfigPath()
		if err != nil {
			fmt.Printf("❌ Error resolving kubeconfig: %v\n", err)
			os.Exit(1)
		}

		ns := p.Project
		if ns == "" {
			ns = p.Namespace
		}
		if ns == "" {
			ns = "kargo"
		}

		svc := gitops.NewKargoService(kubeconfig, config.IsDryRun())
		out, err := svc.Promote(cmd.Context(), ns, stageName, freightID)
		if err != nil {
			fmt.Printf("❌ Error promoting: %v\n%s\n", err, out)
			os.Exit(1)
		}
		fmt.Printf("✅ Promotion to %s started successfully!\n%s\n", stageName, out)
	},
}

var kargoFreightCmd = &cobra.Command{
	Use:   "freight",
	Short: "List available freight IDs",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		pipelineFlag, _ := cmd.Flags().GetString("pipeline")
		p, err := getPipeline(cfg, pipelineFlag)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}

		kubeconfig, err := cfg.K3s.ExpandedKubeconfigPath()
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}

		ns := p.Project
		if ns == "" {
			ns = p.Namespace
		}
		if ns == "" {
			ns = "kargo"
		}

		svc := gitops.NewKargoService(kubeconfig, config.IsDryRun())
		freightList, err := svc.GetFreight(cmd.Context(), ns)
		if err != nil {
			fmt.Printf("❌ Error fetching freight: %v\n", err)
			os.Exit(1)
		}

		if len(freightList) == 0 {
			fmt.Println("ℹ️ No freight available yet. Wait for Warehouse to sync.")
			return
		}

		fmt.Printf("📦 Available Freight in %s:\n%s\n", ns, strings.Join(freightList, "\n"))
	},
}

var kargoSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize Kargo resources (Project, Warehouse, Stages) with local config",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		orch := gitops.NewOrchestrator()
		orch.Output = cmd.OutOrStdout()
		orch.DryRun = config.IsDryRun()

		if err := orch.Sync(cfg); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Kargo synchronization completed.")
	},
}

var kargoInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Kargo configuration with default values",
	Run: func(cmd *cobra.Command, args []string) {
		_ = config.ModifyConfig(func(c *config.ClusterConfig) {
			if len(c.GitOps.Pipelines) == 0 {
				key, _ := config.GenerateRandomString(32)
				c.GitOps.KargoEngine = &config.KargoEngine{
					TokenSigningKey: config.Secret(key),
				}
				c.GitOps.Pipelines = []config.KargoPipeline{
					{
						Name:      "main",
						Namespace: "kargo",
						Project:   "homelab",
						Warehouse: config.KargoWarehouse{
							Name: "default",
						},
					},
				}
			}
		})
		if err := config.SaveConfig(); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Kargo initialized in kuargogo.yaml. You can now edit it or run 'kgg kargo sync'.")
	},
}

var kargoSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set Kargo configuration values",
	Run: func(cmd *cobra.Command, args []string) {
		ns, _ := cmd.Flags().GetString("namespace")
		project, _ := cmd.Flags().GetString("project")
		repo, _ := cmd.Flags().GetString("repo")
		additionalImagesStr, _ := cmd.Flags().GetString("additional-images")
		path, _ := cmd.Flags().GetString("path")
		semver, _ := cmd.Flags().GetString("semver")
		stagesStr, _ := cmd.Flags().GetString("stages")
		pipelineFlag, _ := cmd.Flags().GetString("pipeline")

		_ = config.ModifyConfig(func(c *config.ClusterConfig) {
			if len(c.GitOps.Pipelines) == 0 {
				c.GitOps.Pipelines = append(c.GitOps.Pipelines, config.KargoPipeline{Name: "main"})
			}

			index := 0
			if pipelineFlag != "" {
				found := false
				for i, pipe := range c.GitOps.Pipelines {
					if pipe.Name == pipelineFlag {
						index = i
						found = true
						break
					}
				}
				if !found {
					c.GitOps.Pipelines = append(c.GitOps.Pipelines, config.KargoPipeline{Name: pipelineFlag})
					index = len(c.GitOps.Pipelines) - 1
				}
			}
			k := &c.GitOps.Pipelines[index]

			if ns != "" {
				k.Namespace = ns
			}
			if project != "" {
				k.Project = project
			}
			if repo != "" {
				k.Warehouse.Repo = repo
			}
			if additionalImagesStr != "" {
				k.Warehouse.AdditionalImages = nil
				for _, img := range strings.Split(additionalImagesStr, ",") {
					cleanImg := strings.TrimSpace(img)
					if cleanImg != "" {
						k.Warehouse.AdditionalImages = append(k.Warehouse.AdditionalImages, cleanImg)
					}
				}
			}
			if path != "" {
				k.Warehouse.Path = path
			}
			if semver != "" {
				k.Warehouse.Semver = semver
			}

			if stagesStr != "" {
				k.Stages = nil
				var prevStage string
				for _, s := range strings.Split(stagesStr, ",") {
					parts := strings.Split(strings.TrimSpace(s), ":")
					name := parts[0]
					stagePath := ""
					if len(parts) > 1 {
						stagePath = parts[1]
					}
					if name != "" {
						stage := config.KargoStage{
							Name: name,
							Path: stagePath,
						}
						if prevStage != "" {
							stage.Requires = []string{prevStage}
						}
						k.Stages = append(k.Stages, stage)
						prevStage = name
					}
				}
			}
		})
		if err := config.SaveConfig(); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Kargo configuration updated.")
	},
}

var kargoStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show live Freight and Stage status of a pipeline",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		pipelineFlag, _ := cmd.Flags().GetString("pipeline")
		p, err := getPipeline(cfg, pipelineFlag)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}

		kubeconfig, err := cfg.K3s.ExpandedKubeconfigPath()
		if err != nil {
			fmt.Printf("❌ Error resolving kubeconfig: %v\n", err)
			os.Exit(1)
		}

		svc := gitops.NewKargoService(kubeconfig, config.IsDryRun())
		snapshot, err := svc.QueryObservability(cmd.Context(), p.Name)
		if err != nil {
			fmt.Printf("❌ Error querying observability: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("🚢 KARGO PIPELINE OBSERVABILITY")
		fmt.Printf("Pipeline: %s  ·  Project: %s  ·  Namespace: %s\n", snapshot.PipelineName, snapshot.Project, snapshot.Namespace)
		fmt.Printf("Warehouse: %s\n\n", snapshot.WarehouseName)

		fmt.Println("🛣️  Stages Pipeline Flow:")
		for _, stage := range snapshot.Stages {
			freightStr := stage.CurrentFreight
			if freightStr == "" {
				freightStr = "None"
			}

			alias := ""
			for _, f := range snapshot.Freights {
				if f.Name == stage.CurrentFreight {
					alias = f.Alias
					break
				}
			}
			if alias != "" {
				freightStr = fmt.Sprintf("%s (%s)", freightStr, alias)
			}

			healthIcon := "⚪"
			if strings.EqualFold(stage.HealthStatus, "Healthy") {
				healthIcon = "🟢"
			} else if strings.EqualFold(stage.HealthStatus, "Unhealthy") || strings.EqualFold(stage.HealthStatus, "Degraded") {
				healthIcon = "🔴"
			} else if stage.HealthStatus != "Unknown" && stage.HealthStatus != "" {
				healthIcon = "🟡"
			}

			fmt.Printf("  • Stage: %s\n", strings.ToUpper(stage.Name))
			fmt.Printf("    📦 Freight: %s\n", freightStr)
			fmt.Printf("    ❤️  Health:  %s %s\n", healthIcon, stage.HealthStatus)

			var matchedApp *gitops.ArgoAppSnapshot
			for _, app := range snapshot.ArgoApps {
				if strings.Contains(strings.ToLower(app.Name), strings.ToLower(stage.Name)) {
					matchedApp = &app
					break
				}
			}
			if matchedApp != nil {
				argoHealthIcon := "⚪"
				if strings.EqualFold(matchedApp.HealthStatus, "Healthy") {
					argoHealthIcon = "🟢"
				} else if strings.EqualFold(matchedApp.HealthStatus, "Degraded") || strings.EqualFold(matchedApp.HealthStatus, "Missing") {
					argoHealthIcon = "🔴"
				} else if matchedApp.HealthStatus != "" {
					argoHealthIcon = "🟡"
				}

				syncIcon := "⚪"
				if strings.EqualFold(matchedApp.SyncStatus, "Synced") {
					syncIcon = "🟢"
				} else if strings.EqualFold(matchedApp.SyncStatus, "OutOfSync") {
					syncIcon = "🟡"
				}

				fmt.Printf("    🚢 ArgoCD:  %s %s\n", argoHealthIcon, matchedApp.HealthStatus)
				fmt.Printf("       Sync:    %s %s\n", syncIcon, matchedApp.SyncStatus)
			} else {
				fmt.Println("    🚢 ArgoCD:  ⚪ Missing")
			}
			fmt.Println()
		}

		fmt.Println("📦 Available Freight (Warehouse):")
		if len(snapshot.Freights) == 0 {
			fmt.Println("  No freight available.")
		} else {
			for _, f := range snapshot.Freights {
				aliasStr := ""
				if f.Alias != "" {
					aliasStr = fmt.Sprintf("[%s]", f.Alias)
				}
				imageInfo := ""
				if f.ImageRepo != "" {
					imageInfo = fmt.Sprintf(" [%s:%s]", f.ImageRepo, f.ImageTag)
				}
				activeStr := ""
				if len(f.ActiveInStages) > 0 {
					var uActive []string
					for _, stg := range f.ActiveInStages {
						uActive = append(uActive, strings.ToUpper(stg))
					}
					activeStr = fmt.Sprintf(" (Active: %s)", strings.Join(uActive, ", "))
				}

				nameToTrunc := f.Name
				if len(nameToTrunc) > 12 {
					nameToTrunc = nameToTrunc[:12]
				}
				fmt.Printf("  %-18s %-12s  %s%s%s\n", aliasStr, nameToTrunc, f.CreationTime.Local().Format("2006-01-02 15:04"), imageInfo, activeStr)
			}
		}
	},
}

var kargoPipelinesCmd = &cobra.Command{
	Use:   "pipelines",
	Short: "List all configured Kargo pipeline names",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		for _, pipe := range cfg.GitOps.Pipelines {
			fmt.Println(pipe.Name)
		}
	},
}

var kargoStagesCmd = &cobra.Command{
	Use:   "stages",
	Short: "List all stages for a Kargo pipeline",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		pipelineFlag, _ := cmd.Flags().GetString("pipeline")
		p, err := getPipeline(cfg, pipelineFlag)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
		for _, stage := range p.Stages {
			fmt.Println(stage.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(kargoCmd)
	gitopsCmd.AddCommand(kargoCmd)
	kargoCmd.AddCommand(kargoPromoteCmd)
	kargoCmd.AddCommand(kargoFreightCmd)
	kargoCmd.AddCommand(kargoSyncCmd)
	kargoCmd.AddCommand(kargoInitCmd)
	kargoCmd.AddCommand(kargoSetCmd)
	kargoCmd.AddCommand(kargoStatusCmd)
	kargoCmd.AddCommand(kargoPipelinesCmd)
	kargoCmd.AddCommand(kargoStagesCmd)

	kargoCmd.PersistentFlags().StringP("pipeline", "p", "", "Kargo pipeline name")

	kargoSetCmd.Flags().String("namespace", "", "Kargo namespace")
	kargoSetCmd.Flags().String("project", "", "Kargo project name")
	kargoSetCmd.Flags().String("repo", "", "Warehouse main Image Repository (OCI/Git)")
	kargoSetCmd.Flags().String("additional-images", "", "Comma separated additional images")
	kargoSetCmd.Flags().String("path", "", "Git Ops Repository URL (Warehouse Path)")
	kargoSetCmd.Flags().String("semver", "", "Semver constraint (e.g. ^1.0.0)")
	kargoSetCmd.Flags().String("stages", "", "Comma separated stages (format: name:path)")
}
