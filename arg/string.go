package arg

// String is a argument that takes a string.
type String = Base[string, StringParser]

// StringParser takes the text as it is.
type StringParser struct{}

func (StringParser) Parse(rest []string) (string, int, error) {
	return rest[0], 1, nil
}

func (StringParser) String() string {
	return "string"
}
