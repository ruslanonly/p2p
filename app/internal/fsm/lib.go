package fsm

import "fmt"

func EnterStateFSMCallbackName(state string) string {
	return fmt.Sprintf("enter_%s", state)
}

func LeaveStateFSMCallbackName(state string) string {
	return fmt.Sprintf("leave_%s", state)
}