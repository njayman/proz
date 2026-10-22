package cmd

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/njayman/proz/core"
	"github.com/njayman/proz/mcpserver"
	"github.com/njayman/proz/utils"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Serve proz to agents over MCP (stdio)",
	Long: "Serve proz to an agent over MCP on stdin and stdout. The agent starts it\n" +
		"as a child process; there is no network listener. Tools: list_projects,\n" +
		"get_project, record_milestone, capture_idea, list_nudges.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		open := func() (*core.Store, error) { return core.Open(utils.GetConfigFolder(), time.Now) }
		return mcpserver.New(open, rootCmd.Version).Run(context.Background(), &mcp.StdioTransport{})
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
