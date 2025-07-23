package xli

type CommandOption func(c *Command)

func WithSubcommands(f func() Commands) CommandOption {
	return func(c *Command) {
		c.get_subcommands = f
	}
}
