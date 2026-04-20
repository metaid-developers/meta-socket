package adapter

func NewDOGEZMQAdapter(endpoint, topic string, options ...JSONZMQAdapterOption) ChainZMQAdapter {
	return NewJSONZMQAdapter("doge", endpoint, topic, options...)
}
