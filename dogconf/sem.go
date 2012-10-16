// Semantic Analyzer for dogconf
//
// Converts AST into structures that might be able to be applied onto
// a run-time state.  They are still subject to error, such as an OCN
// clash, and that can only be resolved at execution time.
package dogconf

import (
	"fmt"
	"strconv"
)

// Returned when a wrong-in-all-situations type of target is included
// with an action.  For example: a 'patch' request without an
// OCN-augmented target, or a 'create' given the 'all' target.
type ErrBadTarget struct {
	error
}

// Return error values decorated with token positioning information
func semErrf(blam Blamer, format string, args ...interface{}) error {
	return fmt.Errorf("%s: %s",
		blam.Blame().Pos,
		fmt.Sprintf(format, args))
}

func Analyze(req *RequestSyntax) (Directive, error) {
	switch a := req.Action.(type) {
	case *PatchActionSyntax:
		return analyzePatch(req, a)
	case *CreateActionSyntax:
		//		return analyzeCreate(req, a)
		return nil, nil
	case *GetActionSyntax:
		//		return analyzeGet(req, a)
		return nil, nil
	case *DeleteActionSyntax:
		//		return analyzeDelete(req, a)
		return nil, nil
	}

	panic(fmt.Errorf("Attempting to semantically analyze "+
		"un-enumerated action type %T", req.Action))
}

func initAttrChange(change *AttrChange, props map[*Token]*Token) error {
	// Transform and verify a mapping of tokens to a structured
	// representation.
	for name, value := range props {
		switch name.Lexeme {
		case "addr":
			// KLUDGE: strip lexeme surrounding quotes,
			// and presume those quotes only occupy one
			// character -- e.g. dollar quotes cannot be
			// supported here.
			ipText := value.Lexeme[1 : len(value.Lexeme)-1]

			var err error

			change.Addr, err = hostPortToAddr("tcp", ipText)
			if err != nil {
				return err
			}

			change.AddrBlame = value
		case "dbnameIn":
			change.DbnameIn = value.Lexeme
			change.DbnameInBlame = value
		case "dbnameRewritten":
			change.DbnameRewritten = value.Lexeme
			change.DbnameRewrittenBlame = value
		case "lock":
			// More kludge around the lexeme containing
			// its surrounding quotes.
			switch value.Lexeme {
			case "'true'":
				change.Lock = true
			case "'false'":
				change.Lock = false
			default:
				// ...notably here it is useful to
				// contain the lexeme's surrounding
				// quotes when echoing to the user a
				// mistake.
				err := fmt.Errorf("Could not recognize "+
					"lock literal %v, choose 'true' "+
					"or 'false'", value.Lexeme)
				return err
			}

			change.LockBlame = value
		default:
			// If triggered by a bogus token:
			// theoretically thought to stopped by the
			// lexer.  If an otherwise legitimate token,
			// implement support for it.
			panic(fmt.Sprintf("Unhandleable token: %v", name))
		}
	}

	return nil
}

func newPatchDirective(
	ocnSpec *TargetOcnSpecSyntax,
	ocnInt uint64,
	a *PatchActionSyntax) (d *PatchDirective, err error) {
	var tOcn TargetOcn

	tOcn.Blamer = ocnSpec
	tOcn.TargetOne = TargetOne{ocnSpec, ocnSpec.Ocn.Lexeme}
	tOcn.Ocn = ocnInt

	pd := PatchDirective{
		Blamer:    a,
		TargetOcn: tOcn,
	}

	if err := initAttrChange(&pd.Change, a.PatchProps); err != nil {
		return nil, err
	}

	return &pd, nil
}

func analyzePatch(req *RequestSyntax, a *PatchActionSyntax) (
	d *PatchDirective, err error) {
	var ocnSpec *TargetOcnSpecSyntax
	var ocnInt uint64

	switch t := req.Spec.(type) {
	case *TargetAllSpecSyntax:
		goto badType
	case *TargetOneSpecSyntax:
		goto badType
	case *TargetOcnSpecSyntax:
		ocnSpec = t
	default:
		goto badType
	}

	ocnInt, err = strconv.ParseUint(ocnSpec.Ocn.Lexeme, 10, 64)
	if err != nil {
		return nil, semErrf(ocnSpec.Ocn,
			"Could not parse OCN, %v", err)
	}

	return newPatchDirective(ocnSpec, ocnInt, a)

badType:
	return nil, &ErrBadTarget{
		semErrf(req.Spec, "Incorrect target type: expected "+
			"TargetOcnSpecSyntax, got %T", req.Spec),
	}
}
