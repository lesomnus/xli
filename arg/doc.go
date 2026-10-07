// Package arg is the positional arguments of an xli command: typed, declared
// in order on the command they belong to, and read back by name.
//
//	Args: arg.Args{
//		&arg.String{Name: "SRC"},
//		&arg.String{Name: "DST", Optional: true},
//		&arg.RestStrings{Name: "MORE"}, // variadic, and so optional
//	},
//
// Arguments come after the command's flags; a flag after one of them is an
// error. An argument is required unless it is Optional, and only the last may
// be variadic.
//
// As with a flag, Default is the value you configure, which the framework
// never writes, and Value is what the user gave, which only the framework
// writes. [Get], [Visit] and [VisitP] report whether the user gave the
// argument; [MustGet] returns what the user gave, or else the Default.
package arg
