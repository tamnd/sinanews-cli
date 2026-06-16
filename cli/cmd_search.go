package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) searchCmd() *cobra.Command {
	var page int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Sina News (搜索新浪新闻)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			for i, a := range args {
				if i > 0 {
					query += " "
				}
				query += a
			}
			results, err := a.client.Search(cmd.Context(), query, page)
			if err != nil {
				return mapFetchErr(err)
			}
			if len(results) == 0 {
				return codeError(exitNoData, fmt.Errorf("no results for %q", query))
			}
			return a.renderOrEmpty(results, len(results))
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "result page number (1-based)")
	return cmd
}
