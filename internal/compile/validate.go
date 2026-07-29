package compile

import "fmt"

// opArgBytes returns how many operand bytes follow the opcode (0 or 2).
func opArgBytes(op Op) int {
	switch op {
	case OpLoadConst, OpLoadLocal, OpStoreLocal,
		OpJump, OpJumpIfFalse, OpJumpIfTrue, OpJumpIfFalsePop, OpJumpIfTruePop,
		OpCall, OpDefer,
		OpMakeList, OpMakeMap, OpMakeStruct,
		OpClose:
		return 2
	default:
		if op > OpClose {
			return -1 // unknown
		}
		return 0
	}
}

// ValidateChunk checks bytecode structural integrity.
//
// It rejects: unknown opcodes, truncated operands, OOB const/local indices,
// jump targets outside the code, and jump targets that are not instruction starts
// (so the IP cannot land mid-operand).
//
// It does not prove stack balance; the VM must still report underflow as errors
// (not panics) for hand-crafted or corrupted code.
func ValidateChunk(ch *Chunk) error {
	if ch == nil {
		return fmt.Errorf("nil chunk")
	}
	if ch.NumLocs < 0 {
		return fmt.Errorf("%s: negative NumLocs", ch.Name)
	}
	if ch.Arity < 0 {
		return fmt.Errorf("%s: negative Arity", ch.Name)
	}
	code := ch.Code
	nConst := len(ch.Consts)
	if len(ch.Lines) != 0 && len(ch.Lines) != len(code) {
		return fmt.Errorf("%s: Lines length %d != Code length %d", ch.Name, len(ch.Lines), len(code))
	}

	// First pass: instruction start offsets.
	starts := map[int]bool{0: true}
	ip := 0
	for ip < len(code) {
		op := Op(code[ip])
		ip0 := ip
		ip++
		argN := opArgBytes(op)
		if argN < 0 {
			return fmt.Errorf("%s: unknown opcode %d at ip %d", ch.Name, op, ip0)
		}
		if ip+argN > len(code) {
			return fmt.Errorf("%s: truncated operand for %s at ip %d", ch.Name, op, ip0)
		}
		ip += argN
		if ip < len(code) {
			starts[ip] = true
		}
	}
	if ip != len(code) {
		return fmt.Errorf("%s: internal validate ip mismatch", ch.Name)
	}

	// Second pass: operands and jump targets.
	ip = 0
	for ip < len(code) {
		op := Op(code[ip])
		ip0 := ip
		ip++
		argN := opArgBytes(op)
		var arg uint16
		if argN == 2 {
			arg = uint16(code[ip])<<8 | uint16(code[ip+1])
			ip += 2
		}
		switch op {
		case OpLoadConst:
			if int(arg) >= nConst {
				return fmt.Errorf("%s: LOAD_CONST index %d out of range (consts=%d) at ip %d", ch.Name, arg, nConst, ip0)
			}
		case OpLoadLocal, OpStoreLocal:
			if ch.NumLocs == 0 || int(arg) >= ch.NumLocs {
				return fmt.Errorf("%s: local index %d out of range (NumLocs=%d) at ip %d", ch.Name, arg, ch.NumLocs, ip0)
			}
		case OpJump, OpJumpIfFalse, OpJumpIfTrue, OpJumpIfFalsePop, OpJumpIfTruePop:
			off := int(int16(arg))
			target := ip + off
			if target < 0 || target > len(code) {
				return fmt.Errorf("%s: jump target %d out of range (code len %d) at ip %d", ch.Name, target, len(code), ip0)
			}
			// target == len(code) is a valid "fall off end" exit
			if target < len(code) && !starts[target] {
				return fmt.Errorf("%s: jump target %d is not an instruction start at ip %d", ch.Name, target, ip0)
			}
		case OpCall, OpDefer, OpMakeList, OpMakeMap, OpMakeStruct, OpClose:
			if arg > 4096 {
				return fmt.Errorf("%s: unreasonable operand %d for %s at ip %d", ch.Name, arg, op, ip0)
			}
		}
	}
	return nil
}

// ValidateProgram validates all function chunks in a compiled program.
func ValidateProgram(prog *Program) error {
	if prog == nil {
		return fmt.Errorf("nil program")
	}
	if prog.Main != nil {
		if ch, ok := prog.Main.Chunk.(*Chunk); ok {
			if err := ValidateChunk(ch); err != nil {
				return err
			}
		}
	}
	for name, fn := range prog.Funcs {
		if fn == nil {
			continue
		}
		ch, ok := fn.Chunk.(*Chunk)
		if !ok {
			continue
		}
		if err := ValidateChunk(ch); err != nil {
			return fmt.Errorf("fn %s: %w", name, err)
		}
	}
	return nil
}
