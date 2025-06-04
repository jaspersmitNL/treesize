package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tree-size",
	Short: "Tree Size is a CLI tool to visualize disk usage in a folder tree",
	Long:  `Recursively shows the largest files and folders in a tree structure with size summaries.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
