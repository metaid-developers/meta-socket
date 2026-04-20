package adapter

func NewMVCZMQAdapter(endpoint, topic string, options ...JSONZMQAdapterOption) ChainZMQAdapter {
	return NewJSONZMQAdapter("mvc", endpoint, topic, options...)
}
