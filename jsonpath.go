package jsonmatch

// Match compiles and excutes the jsonmatch expression on the underlying
// data
func Match(path string, data interface{}) (*MatchSet, error) {
	expr, err := Parse(path)
	if err != nil {
		return nil, err
	}
	return expr.Match(data)
}
