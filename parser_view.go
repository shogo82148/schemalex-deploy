package schemalex

import (
	"strings"

	"github.com/shogo82148/schemalex-deploy/model"
)

// parseCreateView parses
// `[OR REPLACE] [ALGORITHM = {UNDEFINED | MERGE | TEMPTABLE}] [DEFINER = user] [SQL SECURITY {DEFINER | INVOKER}]
// VIEW [IF NOT EXISTS] view_name [(column_list)] AS select_statement [WITH [CASCADED | LOCAL] CHECK OPTION]`
// after CREATE.
// IF NOT EXISTS is supported by MariaDB.
// https://dev.mysql.com/doc/refman/26.10/en/create-view.html
// https://mariadb.com/kb/en/create-view/
func (p *Parser) parseCreateView(ctx *parseCtx) (*model.View, error) {
	var view model.View

	// OR REPLACE
	if t := ctx.peek(); t.Type == OR {
		ctx.advance()
		ctx.skipWhiteSpaces()
		if t := ctx.next(); t.Type != REPLACE {
			return nil, newParseError(ctx, t, "expected REPLACE")
		}
		view.OrReplace = true
		ctx.skipWhiteSpaces()
	}

	// ALGORITHM = {UNDEFINED | MERGE | TEMPTABLE}
	if t := ctx.peek(); t.Type == ALGORITHM {
		ctx.advance()
		if err := p.parseEqual(ctx); err != nil {
			return nil, err
		}
		switch t := ctx.next(); t.Type {
		case UNDEFINED:
			view.Algorithm = model.ViewAlgorithmUndefined
		case MERGE:
			view.Algorithm = model.ViewAlgorithmMerge
		case TEMPTABLE:
			view.Algorithm = model.ViewAlgorithmTemptable
		default:
			return nil, newParseError(ctx, t, "expected UNDEFINED, MERGE or TEMPTABLE")
		}
		ctx.skipWhiteSpaces()
	}

	// DEFINER = user
	if t := ctx.peek(); t.Type == DEFINER {
		ctx.advance()
		if err := p.parseEqual(ctx); err != nil {
			return nil, err
		}
		definer, err := p.parseDefiner(ctx)
		if err != nil {
			return nil, err
		}
		view.Definer = model.MaybeString{
			Valid: true,
			Value: definer,
		}
		ctx.skipWhiteSpaces()
	}

	// SQL SECURITY {DEFINER | INVOKER}
	if t := ctx.peek(); t.Type == SQL {
		ctx.advance()
		ctx.skipWhiteSpaces()
		if t := ctx.next(); t.Type != SECURITY {
			return nil, newParseError(ctx, t, "expected SECURITY")
		}
		ctx.skipWhiteSpaces()
		switch t := ctx.next(); t.Type {
		case DEFINER:
			view.SQLSecurity = model.ViewSQLSecurityDefiner
		case INVOKER:
			view.SQLSecurity = model.ViewSQLSecurityInvoker
		default:
			return nil, newParseError(ctx, t, "expected DEFINER or INVOKER")
		}
		ctx.skipWhiteSpaces()
	}

	if t := ctx.next(); t.Type != VIEW {
		return nil, newParseError(ctx, t, "expected VIEW")
	}
	ctx.skipWhiteSpaces()

	// IF NOT EXISTS
	if t := ctx.peek(); t.Type == IF {
		ctx.advance()
		if _, err := p.parseIdents(ctx, NOT, EXISTS); err != nil {
			return nil, err
		}
		if view.OrReplace {
			// MariaDB rejects it.
			return nil, newParseError(ctx, t, "OR REPLACE and IF NOT EXISTS cannot be used together")
		}
		view.IfNotExists = true
		ctx.skipWhiteSpaces()
	}

	t := ctx.next()
	if !isIdent(t) {
		return nil, newParseError(ctx, t, "expected IDENT or BACKTICK_IDENT")
	}
	view.Name = t.Ident()
	ctx.skipWhiteSpaces()

	// (column_list)
	if t := ctx.peek(); t.Type == LPAREN {
		ctx.advance()
		if err := p.parseViewColumns(ctx, &view); err != nil {
			return nil, err
		}
		ctx.skipWhiteSpaces()
	}

	if t := ctx.next(); t.Type != AS {
		return nil, newParseError(ctx, t, "expected AS")
	}
	ctx.skipWhiteSpaces()

	if err := p.parseViewSelect(ctx, &view); err != nil {
		return nil, err
	}

	if t, ok := p.eol(ctx); !ok {
		return nil, newParseError(ctx, t, "expected SEMICOLON or EOF")
	}
	return &view, nil
}

// parseEqual parses `=` and the white spaces around it.
func (p *Parser) parseEqual(ctx *parseCtx) error {
	ctx.skipWhiteSpaces()
	if t := ctx.next(); t.Type != EQUAL {
		return newParseError(ctx, t, "expected EQUAL")
	}
	ctx.skipWhiteSpaces()
	return nil
}

// parseDefiner parses the account of the DEFINER clause, e.g. `'root'@'localhost'`, `root@localhost` and `CURRENT_USER`,
// and returns it as it is written, except for the markers of executable comments.
// https://dev.mysql.com/doc/refman/26.10/en/account-names.html
func (p *Parser) parseDefiner(ctx *parseCtx) (string, error) {
	start := ctx.peekPos()
	var markers []*Token // the markers of executable comments in the account
	// skip skips the white spaces and the comments, and records the markers of executable comments.
	skip := func() {
		for {
			t := ctx.peek()
			if t.Type != SPACE && t.Type != COMMENT_IDENT {
				return
			}
			if ctx.isExecutableCommentMarker(t) {
				markers = append(markers, t)
			}
			ctx.advance()
		}
	}
	// optional parses the tokens by f if it reports true, otherwise it rewinds the tokens.
	optional := func(f func() (bool, error)) error {
		idx, n := ctx.idx, len(markers)
		ok, err := f()
		if err != nil {
			return err
		}
		if !ok {
			ctx.idx, markers = idx, markers[:n]
		}
		return nil
	}

	var end int
	t := ctx.next()
	switch {
	case t.Type == IDENT && strings.EqualFold(t.Value, "CURRENT_USER"):
		// CURRENT_USER [()]
		end = ctx.peekPos()
		err := optional(func() (bool, error) {
			skip()
			if ctx.peek().Type != LPAREN {
				return false, nil
			}
			ctx.advance()
			skip()
			if t := ctx.next(); t.Type != RPAREN {
				return false, newParseError(ctx, t, "expected RPAREN")
			}
			end = ctx.peekPos()
			return true, nil
		})
		if err != nil {
			return "", err
		}
	case isAccountNamePart(t):
		// user_name [@host_name]
		// the user name may be a non-reserved keyword, e.g. `view@localhost`.
		end = ctx.peekPos()
		err := optional(func() (bool, error) {
			skip()
			if t := ctx.peek(); t.Type != ILLEGAL || t.Value != "@" {
				return false, nil
			}
			ctx.advance()
			skip()
			host := ctx.next()
			if isAccountNamePart(host) && host.Type != IDENT && !host.Type.IsKeyword() {
				// a quoted host name.
				end = ctx.peekPos()
				return true, nil
			}
			// an unquoted host name, e.g. `localhost` and `127.0.0.1`.
			// it continues up to a white space, because the lexer splits it into some tokens.
			for t := host; ; t = ctx.next() {
				switch t.Type {
				case SPACE, COMMENT_IDENT, SEMICOLON, EOF:
					ctx.rewind()
					if t == host {
						return false, newParseError(ctx, t, "expected the host name of DEFINER")
					}
					return true, nil
				case ILLEGAL:
					if isUnterminatedQuote(t) {
						return false, newParseError(ctx, t, "unterminated quoted string")
					}
				}
				end = ctx.peekPos()
			}
		})
		if err != nil {
			return "", err
		}
	default:
		if isUnterminatedQuote(t) {
			return "", newParseError(ctx, t, "unterminated quoted string")
		}
		return "", newParseError(ctx, t, "expected the user of DEFINER")
	}
	return strings.TrimFunc(removeExecutableCommentMarkers(ctx.input, start, end, markers), isSpace), nil
}

// isAccountNamePart reports whether t can be the user name or the host name of an account.
func isAccountNamePart(t *Token) bool {
	switch t.Type {
	case SINGLE_QUOTE_IDENT, DOUBLE_QUOTE_IDENT:
		return true
	}
	return isIdent(t)
}

// parseViewColumns parses the column list of a view after `(`.
func (p *Parser) parseViewColumns(ctx *parseCtx, view *model.View) error {
	for {
		ctx.skipWhiteSpaces()
		t := ctx.next()
		if !isIdent(t) {
			return newParseError(ctx, t, "expected IDENT or BACKTICK_IDENT")
		}
		view.Columns = append(view.Columns, t.Ident())

		ctx.skipWhiteSpaces()
		switch t := ctx.next(); t.Type {
		case COMMA:
		case RPAREN:
			return nil
		default:
			return newParseError(ctx, t, "expected COMMA or RPAREN")
		}
	}
}

// parseViewSelect parses the select statement of a view up to the end of the statement,
// and the WITH CHECK OPTION clause after it.
// The select statement is kept as it is written, except for the markers of executable comments,
// because SHOW CREATE VIEW and mysqldump return the definition in executable comments.
func (p *Parser) parseViewSelect(ctx *parseCtx, view *model.View) error {
	first := ctx.peek()
	start := ctx.peekPos()
	// end is the end of the last token, excluding the spaces and the comments after it,
	// e.g. a trailing "-- comment" would comment out the semicolon after the statement in the output.
	end := start
	var markers []*Token // the markers of executable comments in the statement
	depth := 0
LOOP:
	for {
		t := ctx.peek()
		significant := true
		switch t.Type {
		case LPAREN:
			depth++
		case RPAREN:
			if depth == 0 {
				return newParseError(ctx, t, "unexpected RPAREN in the select statement of the view")
			}
			depth--
		case SEMICOLON, EOF:
			if depth > 0 {
				return newParseError(ctx, t, "expected RPAREN in the select statement of the view")
			}
			break LOOP
		case WITH:
			if depth == 0 {
				ok, err := p.parseViewCheckOption(ctx, view)
				if err != nil {
					return err
				}
				if ok {
					break LOOP
				}
			}
		case ILLEGAL:
			// the lexer does not support some operators, e.g. `*` and `<`, and they are ILLEGAL tokens.
			// they are permitted in the select statement, except for unterminated quoted strings.
			if isUnterminatedQuote(t) {
				return newParseError(ctx, t, "unterminated quoted string")
			}
		case SPACE:
			significant = false
		case COMMENT_IDENT:
			significant = false
			if ctx.isExecutableCommentMarker(t) {
				markers = append(markers, t)
			}
		}
		ctx.advance()
		if significant {
			end = ctx.peekPos()
		}
	}
	if end == start {
		return newParseError(ctx, first, "expected the select statement of the view")
	}
	view.Select = strings.TrimFunc(removeExecutableCommentMarkers(ctx.input, start, end, markers), isSpace)
	return nil
}

// parseViewCheckOption parses `WITH [CASCADED | LOCAL] CHECK OPTION`.
// It reports false and does not consume any tokens if WITH does not start the clause,
// e.g. `WITH ROLLUP` and common table expressions such as `WITH cascaded AS (...)`.
func (p *Parser) parseViewCheckOption(ctx *parseCtx, view *model.View) (bool, error) {
	idx := ctx.idx
	if t := ctx.next(); t.Type != WITH {
		ctx.idx = idx
		return false, nil
	}
	ctx.skipWhiteSpaces()
	opt := model.ViewCheckOptionCascaded
	switch t := ctx.peek(); t.Type {
	case CASCADED:
		ctx.advance()
		ctx.skipWhiteSpaces()
	case LOCAL:
		opt = model.ViewCheckOptionLocal
		ctx.advance()
		ctx.skipWhiteSpaces()
	}
	if t := ctx.next(); t.Type != CHECK {
		ctx.idx = idx
		return false, nil
	}

	// `WITH [CASCADED | LOCAL] CHECK` is not valid in select statements, so it must be the clause.
	ctx.skipWhiteSpaces()
	if t := ctx.next(); t.Type != OPTION {
		return false, newParseError(ctx, t, "expected OPTION")
	}
	view.CheckOption = opt
	return true, nil
}

// isUnterminatedQuote reports whether t is a quoted string or identifier that is not terminated.
// The lexer emits it as an ILLEGAL token, and stops lexing.
func isUnterminatedQuote(t *Token) bool {
	if t.Type != ILLEGAL || t.Value == "" {
		return false
	}
	switch t.Value[0] {
	case '\'', '"', '`':
		return true
	}
	return false
}
