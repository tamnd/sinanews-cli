package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) articleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "article <url-or-docid>",
		Short: "Fetch a Sina News article (title, body, author, date, tags)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			detail, err := a.client.Article(cmd.Context(), args[0])
			if err != nil {
				return mapFetchErr(err)
			}
			if detail == nil {
				return codeError(exitNoData, fmt.Errorf("no article found for %q", args[0]))
			}
			return a.render(detail)
		},
	}
}
