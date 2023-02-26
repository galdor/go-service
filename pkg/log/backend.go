package log

type BackendType string

const (
	BackendTypeTerminal BackendType = "terminal"
)

type Backend interface {
	Log(Message)
}
