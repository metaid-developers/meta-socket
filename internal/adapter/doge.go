package adapter

func NewDOGEZMQAdapter(endpoint, topic string) ChainZMQAdapter {
	return NewJSONZMQAdapter("doge", endpoint, topic)
}
