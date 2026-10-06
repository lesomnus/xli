// Package cfg loads an xli application's configuration: its own struct, read
// from a YAML file, the environment and command-line flags, in that order of
// precedence over the defaults the struct holds when the loader is made.
//
//	type Config struct {
//		Db   DbConfig   `yaml:"db"`
//		Ldap LdapConfig `yaml:"ldap"`
//	}
//
//	var c = Config{Ldap: LdapConfig{Addr: ":389"}} // the defaults
//	var l = cfg.New("roster", &c)                  // roster.yaml, ROSTER_*
//
//	serve := &xli.Command{
//		Name: "serve",
//		Flags: flg.Flags{
//			cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "listen"}),
//		},
//	}
//	comp := xli.NewCmdCompletion()
//	root := &xli.Command{
//		Flags:    flg.Flags{cfg.ConfigFlag()},
//		Commands: xli.Commands{serve, comp, cfg.NewCmdConfig(l)},
//		Handler:  xli.Chain(cfg.Load(l, comp), xli.RequireSubcommand()),
//	}
//
// Every field is named after its path (`ldap.addr` in the file,
// ROSTER_LDAP_ADDR in the environment); a flag is bound to a field by pointer.
// Every value records where it came from (Loader.Origin). Secrets (Secret,
// SecretOf) may be references to files that are re-read when they change, and
// Loader.Watch reloads the file. See DESIGN.md for the decisions and why.
package cfg
