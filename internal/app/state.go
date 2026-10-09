package app

type ClearState int

const (
	Normal ClearState = iota
	ReadyToClear
)

type ShowMenuState int

const (
	Click ShowMenuState = iota
	RClick
)

const (
	const_max_history uint = 300
)
