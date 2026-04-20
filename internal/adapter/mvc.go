package adapter

func NewMVCZMQAdapter(endpoint, topic string) ChainZMQAdapter {
	return NewJSONZMQAdapter("mvc", endpoint, topic)
}
