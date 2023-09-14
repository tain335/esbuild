package logger

import (
	"fmt"
	"os"
)

func Infof(format string, args ...any) {
	PrintMessageToStderr(os.Args, Msg{
		Kind: Info,
		Data: MsgData{
			Text: fmt.Sprintf(format, args...),
		},
	})
}

func Errorf(format string, args ...any) {
	PrintMessageToStderr(os.Args, Msg{
		Kind: Error,
		Data: MsgData{
			Text: fmt.Sprintf(format, args...),
		},
	})
}

func Deubgf(format string, args ...any) {
	PrintMessageToStderr(os.Args, Msg{
		Kind: Debug,
		Data: MsgData{
			Text: fmt.Sprintf(format, args...),
		},
	})
}

func Wraningf(format string, args ...any) {
	PrintMessageToStderr(os.Args, Msg{
		Kind: Debug,
		Data: MsgData{
			Text: fmt.Sprintf(format, args...),
		},
	})
}

func Clear() {
	Infof("%s", "\033[H\033[2J")
}
