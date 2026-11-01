// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package usim

// The class and instruction bytes that the card's RFM application takes (TS 102 221 clauses 10.1.1 and 10.1.2): of
// the interindustry class on the basic logical channel.
const (
	claUICC         = 0x00
	insSelect       = 0xa4
	insReadBinary   = 0xb0
	insUpdateBinary = 0xd6
	insGetResponse  = 0xc0
)

// The SELECT parameters that the card takes (TS 102 221 clause 11.1.1.2, tables 11.1 and 11.2): by file identifier,
// returning the FCP template or no data.
const (
	selectByFID  = 0x00
	selectFCP    = 0x04
	selectNoData = 0x0c
)

// The status words of the commands (TS 102 221 clause 10.2.1, table 10.16).
const (
	swEndOfFile         = 0x6282
	swWrongLength       = 0x6700
	swNoEFSelected      = 0x6986
	swFileNotFound      = 0x6a82
	swIncorrectP1P2     = 0x6a86
	swWrongParameters   = 0x6b00
	swClassNotSupported = 0x6e00
	swTechnicalProblem  = 0x6f00
	// swResponseBytes is the procedure bytes '61 XX' over T = 0: XX bytes of response data are available, which GET
	// RESPONSE retrieves (TS 102 221 clause 7.3.1.1.5).
	swResponseBytes = 0x6100
	// swWrongLe is the procedure bytes '6C XX' over T = 0: Le exceeds XX, the length of the response data available
	// (TS 102 221 clause 7.3.1.1.5.1).
	swWrongLe = 0x6c00
)

// ef is a transparent EF of ADF.USIM.
type ef struct {
	fid  uint16
	sfi  byte
	data []byte
}

// fcp is the FCP template of the EF, its data objects in the order of TS 102 221 table 11.4: a shareable working EF
// of transparent structure, operational and activated, whose access rules are in record 1 of EF.ARR, of its size and
// SFI.
func (f *ef) fcp() []byte {
	objects := []byte{
		0x82, 0x02, 0x41, 0x21, // File Descriptor: shareable, working EF, transparent; data coding byte '21'.
		0x83, 0x02, byte(f.fid >> 8), byte(f.fid), // File Identifier.
		0x8a, 0x01, 0x05, // Life Cycle Status Integer: operational state, activated.
		0x8b, 0x03, byte(fileARR >> 8), byte(fileARR & 0xff), 0x01, // Security attributes, referenced to expanded format.
		0x80, 0x02, byte(len(f.data) >> 8), byte(len(f.data)), // File size.
		0x88, 0x01, f.sfi << 3, // Short File Identifier, in bits 8 to 4.
	}

	return append([]byte{0x62, byte(len(objects))}, objects...)
}

// fileSystem is ADF.USIM.
type fileSystem struct {
	efs []*ef
}

func newFileSystem(efs ...*ef) *fileSystem {
	return &fileSystem{efs: efs}
}

func (fs *fileSystem) byFID(fid uint16) *ef {
	for _, f := range fs.efs {
		if f.fid == fid {
			return f
		}
	}

	return nil
}

func (fs *fileSystem) bySFI(sfi byte) *ef {
	for _, f := range fs.efs {
		if f.sfi == sfi {
			return f
		}
	}

	return nil
}

// result is the outcome of a command string.
type result struct {
	executed   int
	statusWord uint16
	// data is the response data of the last command executed, when a case 2 command or GET RESPONSE.
	data []byte
}

// compact is the additional response data of r in the compact format (TS 102 226 clause 5.1.2, table 5.1).
func (r result) compact() []byte {
	return append([]byte{byte(r.executed), byte(r.statusWord >> 8), byte(r.statusWord)}, r.data...)
}

// session is a command session of the RFM application: ADF.USIM is the current directory at its start, with no
// current EF (TS 102 226 clauses 4 and 7.3).
type session struct {
	fs      *fileSystem
	current *ef
	// pending is the response data of the last case 4 command, for GET RESPONSE.
	pending []byte
}

// run runs a command string in the compact format: commands coded as T = 0 TPDU commands, one after the other (TS
// 102 226 clause 5.1.1). It stops at the first command whose SW1 is an error; warnings and procedure bytes do not stop
// it (TS 102 226 clause 4).
func (fs *fileSystem) run(commands []byte) result {
	s := &session{fs: fs}
	r := result{statusWord: swOK}

	for len(commands) > 0 {
		// CLA, INS, P1, P2, P3.
		if len(commands) < 5 {
			r.executed++
			r.statusWord, r.data = swWrongLength, nil

			return r
		}

		cla, ins, p1, p2, p3 := commands[0], commands[1], commands[2], commands[3], int(commands[4])
		commands = commands[5:]

		// P3 is Le for a case 2 command, Lc otherwise.
		var data []byte

		if ins != insReadBinary && ins != insGetResponse {
			if len(commands) < p3 {
				r.executed++
				r.statusWord, r.data = swWrongLength, nil

				return r
			}

			data, commands = commands[:p3], commands[p3:]
		}

		r.executed++
		r.data, r.statusWord = s.command(cla, ins, p1, p2, p3, data)

		if !continues(r.statusWord) {
			return r
		}
	}

	return r
}

// continues reports whether a command string goes on after a command of status word sw: after a normal ending, a
// warning or procedure bytes (TS 102 226 clause 4, TS 102 221 clause 10.2.1).
func continues(sw uint16) bool {
	switch byte(sw >> 8) {
	case 0x90, 0x91, 0x92, 0x61, 0x62, 0x63, 0x6c:
		return true
	default:
		return false
	}
}

// command runs a command, of data for a case 3 or 4 command, and returns its response data and status word.
func (s *session) command(cla, ins, p1, p2 byte, p3 int, data []byte) ([]byte, uint16) {
	if cla != claUICC {
		return nil, swClassNotSupported
	}

	if ins != insGetResponse {
		s.pending = nil
	}

	switch ins {
	case insSelect:
		return s.selectFile(p1, p2, data)
	case insReadBinary:
		return s.readBinary(p1, p2, p3)
	case insUpdateBinary:
		return s.updateBinary(p1, p2, data)
	case insGetResponse:
		return s.getResponse(p1, p2, p3)
	default:
		return nil, swInstructionNotFound
	}
}

// selectFile selects an EF of ADF.USIM by its file identifier (TS 102 221 clause 11.1.1). The FCP template it returns
// is response data of a case 4 command, available by GET RESPONSE.
func (s *session) selectFile(p1, p2 byte, data []byte) ([]byte, uint16) {
	if p1 != selectByFID || p2 != selectFCP && p2 != selectNoData {
		return nil, swIncorrectP1P2
	}

	if len(data) != 2 {
		return nil, swWrongLength
	}

	f := s.fs.byFID(uint16(data[0])<<8 | uint16(data[1]))
	if f == nil {
		return nil, swFileNotFound
	}

	s.current = f

	if p2 == selectNoData {
		return nil, swOK
	}

	s.pending = f.fcp()

	return nil, swResponseBytes | uint16(len(s.pending))
}

// target is the EF of READ BINARY or UPDATE BINARY, and the offset in it: with P1 b8 set, the EF of SFI P1 b5 to b1,
// which becomes the current EF, at offset P2; otherwise the current EF, at offset P1 P2 (TS 102 221 clause 11.1.3.2).
func (s *session) target(p1, p2 byte) (*ef, int, uint16) {
	if p1&0x80 == 0 {
		if s.current == nil {
			return nil, 0, swNoEFSelected
		}

		return s.current, int(p1)<<8 | int(p2), swOK
	}

	if p1&0x60 != 0 {
		return nil, 0, swIncorrectP1P2
	}

	f := s.fs.bySFI(p1 & 0x1f)
	if f == nil {
		return nil, 0, swFileNotFound
	}

	s.current = f

	return f, int(p2), swOK
}

// readBinary reads le bytes of the EF from the offset, or all to its end when le is 0 (TS 102 221 clause 11.1.3, TS
// 102 226 clause 5.1.1), with the warning of an end of file reached before le bytes.
func (s *session) readBinary(p1, p2 byte, le int) ([]byte, uint16) {
	f, offset, sw := s.target(p1, p2)
	if sw != swOK {
		return nil, sw
	}

	if offset >= len(f.data) {
		return nil, swWrongParameters
	}

	end := len(f.data)
	if le != 0 && offset+le <= end {
		end = offset + le
	}

	out := append([]byte{}, f.data[offset:end]...)

	if le != 0 && offset+le > len(f.data) {
		return out, swEndOfFile
	}

	return out, swOK
}

// updateBinary writes data at the offset of the EF (TS 102 221 clause 11.1.4).
func (s *session) updateBinary(p1, p2 byte, data []byte) ([]byte, uint16) {
	f, offset, sw := s.target(p1, p2)
	if sw != swOK {
		return nil, sw
	}

	if offset >= len(f.data) {
		return nil, swWrongParameters
	}

	if len(data) == 0 || offset+len(data) > len(f.data) {
		return nil, swWrongLength
	}

	copy(f.data[offset:], data)

	return nil, swOK
}

// getResponse returns the response data of the command just before it (TS 102 221 clause 12.1.1): all of it when le
// is 0 (TS 102 226 clause 5.1.1), le bytes of it otherwise, with '61 XX' when XX bytes remain, or '6C XX' when le
// exceeds the XX bytes available (TS 102 221 clause 7.3.1.1.5.1). Without response data, it is out of sequence:
// "technical problem, no precise diagnosis".
func (s *session) getResponse(p1, p2 byte, le int) ([]byte, uint16) {
	if p1 != 0 || p2 != 0 {
		return nil, swIncorrectP1P2
	}

	if s.pending == nil {
		return nil, swTechnicalProblem
	}

	pending := s.pending

	switch {
	case le == 0 || le == len(pending):
		s.pending = nil
		return pending, swOK
	case le > len(pending):
		return nil, swWrongLe | uint16(len(pending))
	default:
		s.pending = pending[le:]
		return pending[:le], swResponseBytes | uint16(len(s.pending))
	}
}
