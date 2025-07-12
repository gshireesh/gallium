package cmd

import (
	"github.com/spf13/cobra"
)

var dockerDevCmd = &cobra.Command{
	Use:     "dockerDev",
	Aliases: []string{"dd"},
	Short:   "Alias: [dd], Gallium Docker Development Generator Command.",
	Long: `A Fast and Flexible Static Site Generator built with
                love by spf13 and friends in Go.
                Complete documentation is available at http://dockerDev.spf13.com`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}
