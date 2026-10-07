// Package flg is the flags of an xli command: typed, declared on the command
// they belong to, and read back by name.
//
//	Flags: flg.Flags{
//		&flg.Switch{Name: "verbose", Alias: 'v'},
//		&flg.Int{Name: "retries", Default: &three},
//		&flg.Strings{Name: "tag"},
//	},
//
// A flag's Default is the value you configure, which the framework never
// writes; its Value is what the user gave, which only the framework writes.
// [Get], [Visit] and [VisitP] report whether the user gave the flag, and do
// not look at the Default; [MustGet] returns what the user gave, or else the
// Default. [Find], [Lookup], [LookupP] and [MustFind] do the same for a flag
// given to the command or to any command above it.
//
// A flag's Handler is called with each value as the line is parsed, before
// any command's handler runs, in the mode its command's handler will see; see
// [OnRun] and the other On functions.
package flg
