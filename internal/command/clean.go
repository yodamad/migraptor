package command

import "github.com/spf13/cobra"

var Clean = &cobra.Command{
	Use:     "clean",
	Aliases: []string{"cl"},
	Short:   "Clean images from registries",
	Run: func(cmd *cobra.Command, args []string) {
		cleanImages()
	},
}

func cleanImages() {

}
