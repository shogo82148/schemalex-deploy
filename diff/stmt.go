package diff

import (
	"fmt"
	"io"
)

// Stmt is an SQL statement.
type Stmt string

func (s Stmt) String() string {
	return string(s)
}

// Stmts is a list of diff.Stmt.
type Stmts []Stmt

// Append appends a Stmt to the list.
func (stmts *Stmts) Append(s Stmt) Stmts {
	*stmts = append(*stmts, s)
	return *stmts
}

// WriteTo writes the statements to dst.
func (stmts Stmts) WriteTo(dst io.Writer) (int64, error) {
	eol := []byte(";\n")
	var wrote int64

	for _, s := range stmts {
		n, err := io.WriteString(dst, s.String())
		wrote += int64(n)
		if err != nil {
			return wrote, fmt.Errorf("failed to write a statement %q: %w", s.String(), err)
		}

		n, err = dst.Write(eol)
		wrote += int64(n)
		if err != nil {
			return wrote, err
		}
	}

	return wrote, nil
}
