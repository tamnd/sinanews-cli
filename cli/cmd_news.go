package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/sinanews-cli/sinanews"
)

func (a *App) newsCmd() *cobra.Command {
	var channel string
	var page, num int
	cmd := &cobra.Command{
		Use:   "news",
		Short: "List Sina News articles (新浪新闻)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if num <= 0 {
				num = a.effectiveLimit(20)
			}
			articles, err := a.client.News(cmd.Context(), channel, page, num)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(articles, len(articles))
		},
	}
	cmd.Flags().StringVarP(&channel, "channel", "c", "all", "channel: all|domestic|international")
	cmd.Flags().IntVar(&page, "page", 1, "page number (1-based)")
	cmd.Flags().IntVar(&num, "num", 0, "number of articles per page (0 = --limit or 20)")
	return cmd
}

func (a *App) channelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "channels",
		Short: "List available Sina News channels",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ch := sinanews.Channels()
			return a.render(ch)
		},
	}
}
