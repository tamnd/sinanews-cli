package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) hotCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hot",
		Short: "List Sina News hot/trending items (热点新闻)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := a.client.Hot(cmd.Context())
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(items, len(items))
		},
	}
}
