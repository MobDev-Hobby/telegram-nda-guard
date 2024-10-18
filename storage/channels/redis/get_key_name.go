package redis

func (s *Domain) getHashName() string {
	return s.keyPrefix
}
