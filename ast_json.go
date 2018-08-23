package jsonmatch

import "encoding/json"

func (n *Expression) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.root)
}

func (n *pathNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node  string `json:"node"`
		Nodes []node `json:"nodes"`
	}{
		"path",
		n.nodes,
	})
}

func (n *unionNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node  string `json:"node"`
		Nodes []node `json:"nodes"`
	}{
		"union",
		n.nodes,
	})
}

func (n *fieldNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node string `json:"node"`
		Name string `json:"name"`
	}{
		"field",
		n.name,
	})
}

func (n *existingFieldNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node string `json:"node"`
		Name string `json:"name"`
	}{
		"existingField",
		n.name,
	})
}

func (n *stringNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node  string `json:"node"`
		Pos   int    `json:"pos"`
		Value string `json:"value"`
	}{
		"string",
		n.pos,
		n.value,
	})
}

func (n *intNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node  string `json:"node"`
		Value int    `json:"name"`
	}{
		"int",
		n.value,
	})
}

func (n *indexNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node  string `json:"node"`
		Value int    `json:"name"`
	}{
		"index",
		n.value,
	})
}

func (n *floatNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node  string  `json:"node"`
		Value float64 `json:"value"`
	}{
		"float",
		n.value,
	})
}

func (n *wildcardNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node string `json:"node"`
	}{
		"wildcard",
	})
}

func (n *recursiveNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node string `json:"node"`
	}{
		"recursive",
	})
}

func (n *selfNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node string `json:"node"`
	}{
		"self",
	})
}

func (n *filterNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node     string `json:"node"`
		LHS      node   `json:"lhs"`
		RHS      node   `json:"rhs"`
		Operator string `json:"operator"`
	}{
		"filter",
		n.lhs,
		n.rhs,
		n.operator.String(),
	})
}

func (n *sliceNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node           string `json:"node"`
		Start          int    `json:"start"`
		End            int    `json:"end"`
		Step           int    `json:"step"`
		StartSpecified bool   `json:"startSpecified"`
		EndSpecified   bool   `json:"endSpecified"`
		StepSpecified  bool   `json:"stepSpecified"`
	}{
		"slice",
		n.start,
		n.end,
		n.step,
		n.startSpecified,
		n.endSpecified,
		n.stepSpecified,
	})
}
