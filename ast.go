package jsonmatch

// Expression represents a compiled JSONpath epxression
type Expression struct {
	root node
}

type node interface {
	position() int
}

// A list of nodes to match in sequence "a.b.c"
type pathNode struct {
	pos   int
	nodes []node
}

// A node that matches one field in a map, postulating it as a potential field to
// be created if someone attempts to set a value to it later. "foo"
type fieldNode struct {
	pos  int
	name string
}

// A node that matches an existing field in a map, never matching potential fields
// to be created at a later time
type existingFieldNode struct {
	pos  int
	name string
}

// A literal string node as in [name = "John Appleseed"]
type stringNode struct {
	pos   int
	value string
}

// A literal array index node
type indexNode struct {
	pos    int
	sealed bool
	value  int
}

// A literal int as in `[@ == 7]`
type intNode struct {
	pos   int
	value int
}

// A literal float, as in `[@ == 7.2]`
type floatNode struct {
	pos   int
	value float64
}

// A wildcard node matching all keys in a map or all members of an array `*``
type wildcardNode struct {
	pos int
}

// An operator matching all descendants from the current values downwards `..`
type recursiveNode struct {
	pos int
}

// A union of two paths, as in `foo[bar,bat].baz` or `array[1,3,5:9]`
type unionNode struct {
	pos   int
	nodes []node
}

// A filter to select values among array members, as in `[foo == "bar"]`
type filterNode struct {
	pos      int
	lhs      node
	rhs      node
	operator Token
}

// Basically a noop placeholder for @ or $
type selfNode struct {
	pos int
}

// A slice selector on the form `array[1:6:2]` meaning from index 1, to (not including) index 5
// with a step of 2 (every other)
type sliceNode struct {
	pos            int
	start          int
	end            int
	step           int
	startSpecified bool
	endSpecified   bool
	stepSpecified  bool
}

func (n *stringNode) position() int        { return n.pos }
func (n *sliceNode) position() int         { return n.pos }
func (n *pathNode) position() int          { return n.pos }
func (n *fieldNode) position() int         { return n.pos }
func (n *existingFieldNode) position() int { return n.pos }
func (n *intNode) position() int           { return n.pos }
func (n *floatNode) position() int         { return n.pos }
func (n *wildcardNode) position() int      { return n.pos }
func (n *recursiveNode) position() int     { return n.pos }
func (n *unionNode) position() int         { return n.pos }
func (n *filterNode) position() int        { return n.pos }
func (n *selfNode) position() int          { return n.pos }
func (n *indexNode) position() int         { return n.pos }
