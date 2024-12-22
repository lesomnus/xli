package internal

type FlagTagger[T any] interface {
	flagTag(T)
}

type FlagTag[T any] struct{}

func (FlagTag[T]) flagTag(T) {}
