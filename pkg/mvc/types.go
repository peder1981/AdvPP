package mvc

// EventHandler represents an event handler function
type EventHandler struct {
	EventName string
	Handler   func(interface{}, map[string]interface{}) error
}
