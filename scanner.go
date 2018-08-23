package jsonmatch

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"unicode"
)

// Scanner scans a jsonmatch expression
type Scanner struct {
	r   *bufio.Reader
	pos int
}

// NewScanner creates a new Scanner(!)
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{r: bufio.NewReader(r)}
}

const eof = rune(0)

func (s *Scanner) read() rune {
	ch, _, err := s.r.ReadRune()
	s.pos++
	if err != nil {
		return eof
	}
	return ch
}

func (s *Scanner) unread() {
	_ = s.r.UnreadRune()
	s.pos--
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch rune) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isStartNumberCharacter(ch rune) bool {
	return isDigit(ch) || ch == '-'
}

func isIdentifierCharacter(ch rune) bool {
	return isLetter(ch) || isDigit(ch) || (ch == '_') || (ch == '$')
}

func isIdentifierStartCharacter(ch rune) bool {
	return isLetter(ch) || (ch == '_') || (ch == '$')
}

func (s *Scanner) scanOperator(ch, next rune) (Token, string) {
	// First try any two letter combo
	str := string([]rune{ch, next})
	result := Illegal
	switch str {
	case "==":
		result = Equals
	case ">=":
		result = GTE
	case "<=":
		result = LTE
	case "!=":
		result = NEQ
	case "..":
		result = DotDot
	}
	if result != Illegal {
		// We had a two-letter token match, so consume that second character
		s.read()
		return result, str
	}
	// No two letter tokens, let's try one-letter tokens
	switch ch {
	case '(':
		return ParenLeft, "("
	case ')':
		return ParenRight, ")"
	case '{':
		return BraceLeft, "{"
	case '}':
		return BraceRight, "}"
	case '[':
		return BracketLeft, "["
	case ']':
		return BracketRight, "]"
	case '.':
		return Dot, "."
	case '<':
		return LT, "<"
	case '>':
		return GT, ">"
	case '!':
		return Not, "!"
	case ',':
		return Comma, ","
	case '*':
		return Asterisk, "*"
	case '?':
		return QuestionMark, "?"
	case '|':
		return Pipe, "|"
	case '@':
		return At, "@"
	case ':':
		return Colon, ":"
	}

	// The dollar token is handled as a keyword by scanIdentifier

	return Illegal, ""
}

func (s *Scanner) scanWhitespace() string {
	ch := s.read()
	result := []rune{ch}
	for unicode.IsSpace(ch) {
		result = append(result, ch)
		ch = s.read()
	}
	s.unread()
	return string(result)
}

// Scan gets the next token of the jsonmatch string
func (s *Scanner) Scan() (Token, string, int) {
	pos := s.pos
	ch := s.read()

	if unicode.IsSpace(ch) {
		s.unread()
		return Whitespace, s.scanWhitespace(), pos
	} else if ch == '\'' || ch == '"' {
		s.unread()
		return s.scanQuotedString(ch)
	} else if isStartNumberCharacter(ch) {
		s.unread()
		return s.scanNumber()
	} else if isIdentifierStartCharacter(ch) {
		s.unread()
		return s.scanIdentifier()
	} else if ch == eof {
		return EOF, "", pos
	}

	next := s.read()
	s.unread()
	if token, text := s.scanOperator(ch, next); token != Illegal {
		return token, text, pos
	}

	return Illegal, string(ch), pos
}

// scanOperator consumes one operator
func (s *Scanner) scanNumber() (Token, string, int) {
	pos := s.pos
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	// buf.WriteRune(s.read())

	var decimalPointSeen = false
	var lastCharacterIsDigit = false

	// Handle negative numbers
	if ch := s.read(); ch == '-' {
		buf.WriteRune(ch)
	} else {
		s.unread()
	}

	for {
		if ch := s.read(); ch == eof {
			break
		} else if ch == '.' && decimalPointSeen {
			s.unread()
			break
		} else if !isDigit(ch) && ch != '.' {
			s.unread()
			break
		} else {
			// If the ch is a '.', we will need to determine whether this is a decimal point or
			// the beginning of a range operator
			if ch == '.' {
				nextCh, _ := s.r.Peek(1)
				if nextCh[0] == '.' {
					// This is a range operator, not a decimal point
					s.unread()
					break
				} else {
					decimalPointSeen = true
				}
			}
			buf.WriteRune(ch)
			lastCharacterIsDigit = isDigit(ch)
		}
	}

	// Must start and end on a digit
	if !lastCharacterIsDigit {
		return Illegal, buf.String(), pos
	}

	if decimalPointSeen {
		return Float, buf.String(), pos
	}

	return Integer, buf.String(), pos
}

// scanQuotedString consumes a quoted string
func (s *Scanner) scanQuotedString(quote rune) (Token, string, int) {
	pos := s.pos
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every string character into the buffer.
	// Backslashes are used as escape characters, following JSON semantics:
	// https://tools.ietf.org/html/rfc7159#section-7
	// JSON does not allow single-quoted strings, so here we follow
	// Javascript semantics and interpret them the same as double quotes.
	var lastCharacterIsQuote bool
	var isEscaped bool
	var unicodeSeq string
	var isUTF16Surrogate bool
	for {
		if ch := s.read(); ch == eof {
			break
		} else if unicodeSeq != "" {
			// This handles a Unicode sequence, which can be \uXXXX for UTF-8
			// or UTF-16 characters, or \uXXXX\uXXXX for UTF-16 surrogate pairs
			if isHexDigit(ch) {
				unicodeSeq += string(ch)
			} else if isUTF16Surrogate && len(unicodeSeq) == 6 && ch == '\\' {
				unicodeSeq += string(ch)
			} else if isUTF16Surrogate && len(unicodeSeq) == 7 && ch == 'u' {
				unicodeSeq += string(ch)
			} else {
				break // triggers Illegal below
			}
			if len(unicodeSeq) == 4 {
				// Detect UTF-16 surrogate pair 0xD800-0xDFFF
				b, err := hex.DecodeString(unicodeSeq[2:4])
				if err != nil {
					break // triggers Illegal
				}
				isUTF16Surrogate = b[0] >= 0xD8 && b[0] <= 0xDF
			}
			if (len(unicodeSeq) == 6 && !isUTF16Surrogate) ||
				(len(unicodeSeq) == 12 && isUTF16Surrogate) {
				var char string
				err := json.Unmarshal([]byte(`"`+unicodeSeq+`"`), &char)
				if err != nil {
					break // triggers Illegal below
				}
				buf.WriteRune([]rune(char)[0])
				unicodeSeq = ""
				isUTF16Surrogate = false
			}
		} else if isEscaped {
			switch ch {
			case '\\', '/', '\'', '"':
				buf.WriteRune(ch)
			case 'b':
				buf.WriteRune('\u0008')
			case 'f':
				buf.WriteRune('\u000c')
			case 'n':
				buf.WriteRune('\u000a')
			case 'r':
				buf.WriteRune('\u000d')
			case 't':
				buf.WriteRune('\u0009')
			case 'u':
				// Entering Unicode sequence of form \uXXXX where X is hex digit
				unicodeSeq = `\u`
			default:
				break // triggers Illegal below
			}
			isEscaped = false
		} else if ch == rune('\\') {
			isEscaped = true
		} else if ch == quote {
			buf.WriteRune(ch)
			lastCharacterIsQuote = true
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	if !lastCharacterIsQuote {
		return Illegal, buf.String(), pos
	}

	if quote == '\'' {
		return SingleQuotedString, buf.String(), pos
	}
	return DoubleQuotedString, buf.String(), pos
}

// scanIdent consumes the current rune and all contiguous ident runes.
func (s *Scanner) scanIdentifier() (Token, string, int) {
	pos := s.pos
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent ident character into the buffer.
	// Non-ident characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); isIdentifierCharacter(ch) {
			buf.WriteRune(ch)
		} else {
			s.unread()
			break
		}
	}

	// If the string matches a keyword then return that keyword.
	switch buf.String() {
	case "$":
		return Dollar, "$", pos
	case "true":
		return Bool, buf.String(), pos
	case "false":
		return Bool, buf.String(), pos
	}

	// Otherwise return as a regular identifier.
	return Identifier, buf.String(), pos
}
