// Package cfg loads an xli application's configuration: its own struct, read
// from a YAML file, the environment and command-line flags, in that order of
// precedence over defaults.
//
//	type Config struct {
//		Db   DbConfig   `yaml:"db"`
//		Ldap LdapConfig `yaml:"ldap"`
//	}
//
//	var c Config
//	var l = cfg.New("roster", &c) // roster.yaml, ROSTER_*
//
//	root := &xli.Command{
//		Flags:    flg.Flags{cfg.ConfigFlag()},
//		Commands: xli.Commands{serve, cfg.NewCmdConfig(l)},
//		Handler:  xli.Chain(cfg.Load(l), xli.RequireSubcommand()),
//	}
//	serve := &xli.Command{
//		Flags: flg.Flags{
//			cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "listen"}),
//		},
//	}
//
// Every field is named after its path (`ldap.addr` in the file,
// ROSTER_LDAP_ADDR in the environment); a flag is bound to a field by pointer.
// Every value records where it came from (Loader.Origin). Secrets (Secret,
// SecretOf) may be references to files that are re-read when they change, and
// Loader.Watch reloads the file. See DESIGN.md for the decisions and why.
package cfg
