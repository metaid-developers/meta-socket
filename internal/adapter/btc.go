package adapter

func NewBTCZMQAdapter(endpoint, topic string, options ...JSONZMQAdapterOption) ChainZMQAdapter {
	return NewJSONZMQAdapter("btc", endpoint, topic, options...)
}
