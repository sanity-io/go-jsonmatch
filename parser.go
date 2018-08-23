package jsonmatch

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// ParseError describes an error in the parse and contains the position
// of the problem along with the message.
type ParseError struct {
	Message string
	Pos     int
}

func (e *ParseError) Error() string {
	return e.Message
}

// Parser represents a JSONpath parser
type Parser struct {
	s   *Scanner
	buf struct {
		tok Token
		lit string
		pos int
		n   int
	}
}

// MustParse parses a JSONPath expression, and panics on failure.
func MustParse(src string) *Expression {
	expr, err := Parse(src)
	if err != nil {
		panic(fmt.Sprintf("Could not parse %q: %s", src, err))
	}
	return expr
}

// NewParser returns a new instance of Parser.
func NewParser(r io.Reader) *Parser {
	return &Parser{s: NewScanner(r)}
}

// Parse the provided string returning its compiled representation
func Parse(src string) (*Expression, error) {
	return NewParser(bytes.NewReader([]byte(src))).Parse()
}

// Parse executes the parser
func (p *Parser) Parse() (*Expression, error) {
	result, any, err := p.parsePath()
	if err != nil {
		return nil, err
	}
	tok, _, pos := p.scan()
	if tok != EOF {
		return nil, &ParseError{
			Pos:     pos,
			Message: "Syntax error, unable to parse entire expression",
		}
	}

	if !any {
		result = &selfNode{pos: 0}
	}
	return &Expression{root: result}, nil
}

// scan returns the next token from the underlying scanner.
// If a token has been unscanned then read that instead.
func (p *Parser) scanMindfulOfWhitespace() (tok Token, lit string, pos int) {
	// If we have a token on the buffer, then return it.
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit, p.buf.pos
	}

	// Otherwise read the next token from the scanner.
	tok, lit, pos = p.s.Scan()

	// Save it to the buffer in case we unscan later.
	p.buf.tok, p.buf.lit, p.buf.pos = tok, lit, pos

	return
}

// scan the next non-whitespace token.
func (p *Parser) scan() (tok Token, lit string, pos int) {
	tok, lit, pos = p.scanMindfulOfWhitespace()
	if tok == Whitespace {
		tok, lit, pos = p.scanMindfulOfWhitespace()
	}
	return
}

func unwrapIfSingleNodeList(in node) node {
	// Unwrap single nodes
	if path, ok := in.(*pathNode); ok {
		if len(path.nodes) == 1 {
			return path.nodes[0]
		}
		return path
	}
	if union, ok := in.(*unionNode); ok {
		if len(union.nodes) == 1 {
			return union.nodes[0]
		}
		return union
	}
	return in
}

// unscan pushes the previously read token back onto the buffer.
func (p *Parser) unscan() { p.buf.n = 1 }

// top level expression parser
func (p *Parser) parseExpression() (node, error) {
	// We need to establish a position
	_, _, pos := p.scan()
	p.unscan()
	result := &unionNode{pos: pos}

	done := false
	for !done {
		path, any, err := p.parsePath()
		if err != nil {
			return nil, err
		}
		if any {
			result.nodes = append(result.nodes, path)
		}

		token, literal, pos := p.scan()
		switch token {
		case Whitespace:
			// ignore
		case Comma:
			// comma is good, nothing to do
		case Colon:
			// colon means we are building an array node
			p.unscan()
			if len(result.nodes) < 1 {
				// An array selector starting with a colon implies selecting from start of array
				expr, err := p.parseSliceExpression(nil)
				if err != nil {
					return nil, err
				}
				result.nodes = append(result.nodes, expr)
			} else {
				// An array selector following an integer means we were allready building the slice
				// expression, we just didn't know because of our very limited lookahed. No problem,
				// we'll just substitute the last node for a slice expression.
				sliceExprIndex := len(result.nodes) - 1
				expr, err := p.parseSliceExpression(result.nodes[sliceExprIndex])
				if err != nil {
					return nil, err
				}
				result.nodes[sliceExprIndex] = expr
			}
			// The operators:
		case Equals:
			fallthrough
		case GT:
			fallthrough
		case GTE:
			fallthrough
		case LT:
			fallthrough
		case LTE:
			fallthrough
		case NEQ:
			if len(result.nodes) < 1 {
				return nil, &ParseError{
					Pos:     pos,
					Message: fmt.Sprintf("Operator %v require a left hand side operand", token),
				}
			}
			lhs := result.nodes[len(result.nodes)-1]
			filter, err := p.parseFilter(lhs, token)
			if err != nil {
				return nil, err
			}
			// replace the last node in the union with the filter
			result.nodes[len(result.nodes)-1] = filter
		case Illegal:
			return nil, &ParseError{
				Pos:     pos,
				Message: fmt.Sprintf("Syntax error. (Illegal token %q)", literal),
			}
		default:
			p.unscan()
			done = true
		}
	}
	return seal(unwrapIfSingleNodeList(result)), nil
}

func (p *Parser) parseSliceExpression(first node) (node, error) {
	_, _, pos := p.scan()
	p.unscan()
	var params []*indexNode
	if first == nil {
		params = []*indexNode{nil}
	} else if firstInt, ok := first.(*indexNode); ok {
		params = []*indexNode{firstInt}
	} else {
		return nil, &ParseError{
			Pos:     pos,
			Message: "A slice operator ':' require integer indicies",
		}
	}
	for {
		token, _, _ := p.scan()
		if token != Colon {
			p.unscan()
			break
		}
		if atom, any := p.parseAtom(); any {
			if n, ok := atom.(*indexNode); ok {
				params = append(params, n)
			} else {
				return nil, &ParseError{
					Pos:     pos,
					Message: "A slice operator ':' require integer indicies",
				}
			}
		} else {
			p.unscan()
			break
		}
	}

	result := &sliceNode{pos: pos}
	if params[0] != nil {
		result.start = params[0].value
		result.startSpecified = true
	}
	if len(params) > 1 && params[1] != nil {
		result.end = params[1].value
		result.endSpecified = true
	}
	if len(params) > 2 && params[2] != nil {
		result.step = params[2].value
		result.stepSpecified = true
	}
	return result, nil
}

func (p *Parser) parsePath() (node, bool, error) {
	_, _, pos := p.scan()
	p.unscan()
	result := &pathNode{pos: pos}
	done := false
	noNakedIntegers := false
	for !done {
		// Some separator is required before next atom:
		token, literal, pos := p.scan()
		switch token {
		case BracketLeft:
			// Check for filter-node marker [?(...)] for backwards compatibility
			hasFilterNodeMarker := false
			token, _, _ = p.scan()
			if token == QuestionMark {
				token, _, pos = p.scan()
				if token == ParenLeft {
					hasFilterNodeMarker = true
				} else {
					return nil, false, &ParseError{
						Message: "Expected '(' after '[?'",
						Pos:     pos,
					}
				}
			} else {
				p.unscan()
			}

			// Parse the innards
			expr, err := p.parseExpression()
			if err != nil {
				return nil, false, err
			}

			// Expect terminating ')' if this started using filter marker '[?('
			if hasFilterNodeMarker {
				token, _, pos = p.scan()
				if token != ParenRight {
					return nil, false, &ParseError{
						Message: "Expected ')]'",
						Pos:     pos,
					}
				}
				if _, isFilterNode := expr.(*filterNode); !isFilterNode {
					// If is not filter node, then this is an implicit exists
					// operator as in [?(has.this.property)]
					expr = &filterNode{lhs: expr, operator: Exists}
				}
			} else {
				// Check for the jsonpath2 exists operator
				token, _, _ = p.scan()
				if token == QuestionMark {
					expr = &filterNode{lhs: expr, operator: Exists}
				} else {
					p.unscan()
				}
			}

			// Expect the terminating ']'
			token, _, pos = p.scan()
			if token != BracketRight {
				return nil, false, &ParseError{
					Message: "']' must appear",
					Pos:     pos,
				}
			}
			result.nodes = append(result.nodes, expr)
		case Dot:
			// After this points, no naked integers allowed
			noNakedIntegers = true
		case DotDot:
			result.nodes = append(result.nodes, &recursiveNode{pos: pos})
			token, text, aPos := p.scan()
			// If the first token following a recursive, is a field, we parse it as a existingFieldNode
			// to guard against malcovich-malcovich scenarios where you want to update all values in a document
			// by setting say `.._weak` to false, but end up adding said key to EVERY object in the document.
			// existingFieldNode is like fieldNode but will only match fields that exists allready
			if token == Identifier {
				result.nodes = append(result.nodes, &existingFieldNode{pos: aPos, name: text})
			} else {
				p.unscan()
			}
		case Illegal:
			return nil, false, &ParseError{
				Pos:     pos,
				Message: fmt.Sprintf("Syntax error. (Illegal token %q)", literal),
			}
		case Integer:
			if noNakedIntegers {
				return nil, false, &ParseError{
					Pos:     pos,
					Message: fmt.Sprintf("Wrap numbers in brackets when used in dotted path expressions ([%s] or [%q] depending on what you mean)", literal, literal),
				}
			}
			fallthrough
		default:
			p.unscan()
			// Parse next atom in path
			if atom, any := p.parseAtom(); any {
				result.nodes = append(result.nodes, atom)
			} else {
				done = true
			}
		}
	}
	if len(result.nodes) == 0 {
		p.unscan()
		return nil, false, nil
	}
	return unwrapIfSingleNodeList(result), true, nil
}

// Parses one complete JSONpath expression
func (p *Parser) parseAtom() (node, bool) {
	token, text, pos := p.scan()
	switch token {
	case Identifier:
		return &fieldNode{pos: pos, name: text}, true
	case SingleQuotedString:
		return &fieldNode{pos: pos, name: text[1 : len(text)-1]}, true
	case DoubleQuotedString:
		return &stringNode{pos: pos, value: text[1 : len(text)-1]}, true
	case Integer:
		val, _ := strconv.Atoi(text)
		return &indexNode{pos: pos, value: val}, true
	case Float:
		val, _ := strconv.ParseFloat(text, 64)
		return &floatNode{pos: pos, value: val}, true
	case Asterisk:
		return &wildcardNode{pos: pos}, true
	case At:
		fallthrough
	case Dollar:
		return &selfNode{pos: pos}, true
	}
	p.unscan()
	return nil, false
}

func (p *Parser) parseFilter(lhs node, operator Token) (node, error) {
	lhs = convertToComparisionOperatorTerm(lhs)
	rhs, any, err := p.parsePath()
	if err != nil {
		return nil, err
	}
	rhs = convertToComparisionOperatorTerm(rhs)
	if !any {
		_, _, pos := p.scan()
		p.unscan()
		return nil, &ParseError{
			Pos:     pos,
			Message: "Expected an operand for the operator",
		}
	}
	return &filterNode{
		lhs:      lhs,
		rhs:      rhs,
		operator: operator,
	}, nil
}

func convertToComparisionOperatorTerm(in node) node {
	switch n := in.(type) {
	case *indexNode:
		if n.sealed {
			return in
		}
		return &intNode{pos: n.pos, value: n.value}
	}
	return in
}

// When nodes leave bracketed expressions, they get sealed in order to
// not get transformed into literals when given to operators.
// 4 > 2 means the literal int 4 is more than 2
// [4] > 2 means the content at array index 4 is more than 2 (4 is sealed)

func seal(in node) node {
	switch n := in.(type) {
	case *indexNode:
		n.sealed = true
	}
	return in
}
