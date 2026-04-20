package adapter

func NewBTCZMQAdapter(endpoint, topic string) ChainZMQAdapter {
	return NewJSONZMQAdapter("btc", endpoint, topic)
}
