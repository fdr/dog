// Semantic Analyzer Types
//
// For mechanics, see sem.go
package dogconf

import (
	"net"
)

// Union of types that describe a kind of target for an action
type Target interface {
	Blamer
}

// Targets everything.  Useful with delete and get.
type TargetAll struct {
	Blamer
}

// Targets a specific record, regardless of OCN -- hence, subject to
// race conditions.
type TargetOne struct {
	Blamer
	What string
}

// The most specific target: targets a specific thing at a specific
// version, as to be able to raise optimistic concurrency violations
// when there is a version/ocn mismatch.
type TargetOcn struct {
	Blamer
	TargetOne
	Ocn uint64
}

// Toplevel emission from semantic analysis: a single semantically
// analyzed action to be interpreted by the executor.
type Directive interface {
	Blamer
}

type PatchDirective struct {
	Blamer
	TargetOcn
	Change AttrChange
}

type CreateDirective struct {
	Blamer
	TargetOne
	Change AttrChange
}

type DeleteDirective struct {
	Target
}

type GetDirective struct {
	// Only valid targets for get: 'all' and targets without ocn
	Target
}

type AttrChange struct {
	// Monolithically handle all possible change requests to a
	// route, instead of using a dynamic data structure and
	// complex higher-order programming to make it more generic
	// for so few elements.
	//
	// This approach trades more boilerplate code for more type
	// checking and a less baroque abstraction.
	//
	// Mechanism: Every change-able field has two fields here: a
	// Blamer and a semantic representation.  The Blamer's use is
	// overloaded:
	//
	//   * When nil, no request to change this field has been
	//     submitted by the user.
	//
	//   * Allows one to raise errors pointing to a particular
	//     element.  This is the normal use of a Blamer.
	//
	//  A limitation of this coupling of the above is that it is
	//  *impossible* to have an effective AttrChange that has no
	//  lexical source location, or at least a completely bogus
	//  non-nil Blamer.
	AddrBlame Blamer
	Addr      net.Addr

	DbnameInBlame Blamer
	DbnameIn      string

	DbnameRewrittenBlame Blamer
	DbnameRewritten      string

	LockBlame Blamer
	Lock      bool
}
