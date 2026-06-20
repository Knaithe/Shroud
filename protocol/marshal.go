package protocol

import "encoding/json"

// HIMess

func (m *HIMess) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.GreetingLen)
	w.putStr(m.Greeting)
	w.putU16(m.UUIDLen)
	w.putStr(m.UUID)
	w.putU16(m.IsAdmin)
	w.putU16(m.IsReconnect)
	return w.Bytes()
}

func (m *HIMess) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.GreetingLen = r.u16()
	m.Greeting = r.str(int(m.GreetingLen))
	m.UUIDLen = r.u16()
	m.UUID = r.str(int(m.UUIDLen))
	m.IsAdmin = r.u16()
	m.IsReconnect = r.u16()
	return r.err
}

// UUIDMess

func (m *UUIDMess) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.UUIDLen)
	w.putStr(m.UUID)
	return w.Bytes()
}

func (m *UUIDMess) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.UUIDLen = r.u16()
	m.UUID = r.str(int(m.UUIDLen))
	return r.err
}

// ChildUUIDReq

func (m *ChildUUIDReq) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.ParentUUIDLen)
	w.putStr(m.ParentUUID)
	w.putU16(m.IPLen)
	w.putStr(m.IP)
	w.putU16(m.WantsEnrollment)
	w.putU16(uint16(len(m.Ed25519Public)))
	w.putBytes(m.Ed25519Public)
	w.putU16(uint16(len(m.X25519Public)))
	w.putBytes(m.X25519Public)
	certData, _ := json.Marshal(m.Cert)
	if len(m.Cert.Signature) == 0 {
		certData = nil
	}
	w.putU32(uint32(len(certData)))
	w.putBytes(certData)
	return w.Bytes()
}

func (m *ChildUUIDReq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.ParentUUIDLen = r.u16()
	m.ParentUUID = r.str(int(m.ParentUUIDLen))
	m.IPLen = r.u16()
	m.IP = r.str(int(m.IPLen))
	if r.err != nil || r.pos == len(data) {
		return r.err
	}
	m.WantsEnrollment = r.u16()
	m.Ed25519PublicLen = r.u16()
	m.Ed25519Public = r.readBytes(int(m.Ed25519PublicLen))
	m.X25519PublicLen = r.u16()
	m.X25519Public = r.readBytes(int(m.X25519PublicLen))
	m.CertLen = r.u32()
	if m.CertLen > 0 {
		certData := r.readBytes(int(m.CertLen))
		if r.err == nil {
			r.err = json.Unmarshal(certData, &m.Cert)
		}
	}
	return r.err
}

// ChildUUIDRes

func (m *ChildUUIDRes) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.UUIDLen)
	w.putStr(m.UUID)
	w.putU16(m.OK)
	w.putU16(uint16(len(m.Error)))
	w.putStr(m.Error)
	respData, _ := json.Marshal(m.EnrollmentResponse)
	if len(m.EnrollmentResponse.AgentCert.Signature) == 0 {
		respData = nil
	}
	w.putU32(uint32(len(respData)))
	w.putBytes(respData)
	return w.Bytes()
}

func (m *ChildUUIDRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.UUIDLen = r.u16()
	m.UUID = r.str(int(m.UUIDLen))
	if r.err != nil || r.pos == len(data) {
		if m.OK == 0 {
			m.OK = 1
		}
		return r.err
	}
	m.OK = r.u16()
	m.ErrorLen = r.u16()
	m.Error = r.str(int(m.ErrorLen))
	m.EnrollmentResponseLen = r.u32()
	if m.EnrollmentResponseLen > 0 {
		respData := r.readBytes(int(m.EnrollmentResponseLen))
		if r.err == nil {
			r.err = json.Unmarshal(respData, &m.EnrollmentResponse)
		}
	}
	return r.err
}

// MyInfo

func (m *MyInfo) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.UUIDLen)
	w.putStr(m.UUID)
	w.putU64(m.UsernameLen)
	w.putStr(m.Username)
	w.putU64(m.HostnameLen)
	w.putStr(m.Hostname)
	w.putU64(m.MemoLen)
	w.putStr(m.Memo)
	return w.Bytes()
}

func (m *MyInfo) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.UUIDLen = r.u16()
	m.UUID = r.str(int(m.UUIDLen))
	m.UsernameLen = r.u64()
	m.Username = r.str(int(m.UsernameLen))
	m.HostnameLen = r.u64()
	m.Hostname = r.str(int(m.HostnameLen))
	m.MemoLen = r.u64()
	m.Memo = r.str(int(m.MemoLen))
	return r.err
}

// MyMemo

func (m *MyMemo) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.MemoLen)
	w.putStr(m.Memo)
	return w.Bytes()
}

func (m *MyMemo) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.MemoLen = r.u64()
	m.Memo = r.str(int(m.MemoLen))
	return r.err
}

// ShellReq

func (m *ShellReq) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.Start)
	return w.Bytes()
}

func (m *ShellReq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Start = r.u16()
	return r.err
}

// ShellRes

func (m *ShellRes) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *ShellRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// ShellCommand

func (m *ShellCommand) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.CommandLen)
	w.putStr(m.Command)
	return w.Bytes()
}

func (m *ShellCommand) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.CommandLen = r.u64()
	m.Command = r.str(int(m.CommandLen))
	return r.err
}

// ShellResult

func (m *ShellResult) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.ResultLen)
	w.putStr(m.Result)
	return w.Bytes()
}

func (m *ShellResult) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.ResultLen = r.u64()
	m.Result = r.str(int(m.ResultLen))
	return r.err
}

// ShellExit

func (m *ShellExit) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *ShellExit) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// ListenReq

func (m *ListenReq) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.Method)
	w.putU64(m.AddrLen)
	w.putStr(m.Addr)
	return w.Bytes()
}

func (m *ListenReq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Method = r.u16()
	m.AddrLen = r.u64()
	m.Addr = r.str(int(m.AddrLen))
	return r.err
}

// ListenRes

func (m *ListenRes) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *ListenRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// SSHReq

func (m *SSHReq) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.Method)
	w.putU16(m.AddrLen)
	w.putStr(m.Addr)
	w.putU64(m.UsernameLen)
	w.putStr(m.Username)
	w.putU64(m.PasswordLen)
	w.putStr(m.Password)
	w.putU64(m.CertificateLen)
	w.putBytes(m.Certificate)
	w.putU16(m.HostKeyFingerprintLen)
	w.putStr(m.HostKeyFingerprint)
	return w.Bytes()
}

func (m *SSHReq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Method = r.u16()
	m.AddrLen = r.u16()
	m.Addr = r.str(int(m.AddrLen))
	m.UsernameLen = r.u64()
	m.Username = r.str(int(m.UsernameLen))
	m.PasswordLen = r.u64()
	m.Password = r.str(int(m.PasswordLen))
	m.CertificateLen = r.u64()
	m.Certificate = r.readBytes(int(m.CertificateLen))
	m.HostKeyFingerprintLen = r.u16()
	m.HostKeyFingerprint = r.str(int(m.HostKeyFingerprintLen))
	return r.err
}

// SSHRes

func (m *SSHRes) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *SSHRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// SSHCommand

func (m *SSHCommand) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.CommandLen)
	w.putStr(m.Command)
	return w.Bytes()
}

func (m *SSHCommand) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.CommandLen = r.u64()
	m.Command = r.str(int(m.CommandLen))
	return r.err
}

// SSHResult

func (m *SSHResult) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.ResultLen)
	w.putStr(m.Result)
	return w.Bytes()
}

func (m *SSHResult) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.ResultLen = r.u64()
	m.Result = r.str(int(m.ResultLen))
	return r.err
}

// SSHExit

func (m *SSHExit) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *SSHExit) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// SSHTunnelReq

func (m *SSHTunnelReq) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.Method)
	w.putU16(m.AddrLen)
	w.putStr(m.Addr)
	w.putU16(m.PortLen)
	w.putStr(m.Port)
	w.putU64(m.UsernameLen)
	w.putStr(m.Username)
	w.putU64(m.PasswordLen)
	w.putStr(m.Password)
	w.putU64(m.CertificateLen)
	w.putBytes(m.Certificate)
	w.putU16(m.HostKeyFingerprintLen)
	w.putStr(m.HostKeyFingerprint)
	return w.Bytes()
}

func (m *SSHTunnelReq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Method = r.u16()
	m.AddrLen = r.u16()
	m.Addr = r.str(int(m.AddrLen))
	m.PortLen = r.u16()
	m.Port = r.str(int(m.PortLen))
	m.UsernameLen = r.u64()
	m.Username = r.str(int(m.UsernameLen))
	m.PasswordLen = r.u64()
	m.Password = r.str(int(m.PasswordLen))
	m.CertificateLen = r.u64()
	m.Certificate = r.readBytes(int(m.CertificateLen))
	m.HostKeyFingerprintLen = r.u16()
	m.HostKeyFingerprint = r.str(int(m.HostKeyFingerprintLen))
	return r.err
}

// SSHTunnelRes

func (m *SSHTunnelRes) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *SSHTunnelRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// FileStatReq

func (m *FileStatReq) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.TransferID)
	w.putU32(m.FilenameLen)
	w.putStr(m.Filename)
	w.putU64(m.FileSize)
	w.putU64(m.SliceNum)
	return w.Bytes()
}

func (m *FileStatReq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.TransferID = r.u64()
	m.FilenameLen = r.u32()
	m.Filename = r.str(int(m.FilenameLen))
	m.FileSize = r.u64()
	m.SliceNum = r.u64()
	return r.err
}

// FileStatRes

func (m *FileStatRes) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.TransferID)
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *FileStatRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.TransferID = r.u64()
	m.OK = r.u16()
	return r.err
}

// FileData

func (m *FileData) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.TransferID)
	w.putU64(m.DataLen)
	w.putBytes(m.Data)
	return w.Bytes()
}

func (m *FileData) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.TransferID = r.u64()
	m.DataLen = r.u64()
	m.Data = r.readBytes(int(m.DataLen))
	return r.err
}

// FileErr

func (m *FileErr) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.TransferID)
	w.putU16(m.Error)
	return w.Bytes()
}

func (m *FileErr) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.TransferID = r.u64()
	m.Error = r.u16()
	return r.err
}

// FileDownReq

func (m *FileDownReq) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.TransferID)
	w.putU32(m.FilePathLen)
	w.putStr(m.FilePath)
	w.putU32(m.FilenameLen)
	w.putStr(m.Filename)
	return w.Bytes()
}

func (m *FileDownReq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.TransferID = r.u64()
	m.FilePathLen = r.u32()
	m.FilePath = r.str(int(m.FilePathLen))
	m.FilenameLen = r.u32()
	m.Filename = r.str(int(m.FilenameLen))
	return r.err
}

// FileDownRes

func (m *FileDownRes) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.TransferID)
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *FileDownRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.TransferID = r.u64()
	m.OK = r.u16()
	return r.err
}

// SocksStart

func (m *SocksStart) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.UsernameLen)
	w.putStr(m.Username)
	w.putU64(m.PasswordLen)
	w.putStr(m.Password)
	return w.Bytes()
}

func (m *SocksStart) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.UsernameLen = r.u64()
	m.Username = r.str(int(m.UsernameLen))
	m.PasswordLen = r.u64()
	m.Password = r.str(int(m.PasswordLen))
	return r.err
}

// SocksTCPData

func (m *SocksTCPData) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	w.putU64(m.DataLen)
	w.putBytes(m.Data)
	return w.Bytes()
}

func (m *SocksTCPData) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	m.DataLen = r.u64()
	m.Data = r.readBytes(int(m.DataLen))
	return r.err
}

// SocksUDPData

func (m *SocksUDPData) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	w.putU64(m.DataLen)
	w.putBytes(m.Data)
	return w.Bytes()
}

func (m *SocksUDPData) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	m.DataLen = r.u64()
	m.Data = r.readBytes(int(m.DataLen))
	return r.err
}

// UDPAssStart

func (m *UDPAssStart) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	w.putU16(m.SourceAddrLen)
	w.putStr(m.SourceAddr)
	return w.Bytes()
}

func (m *UDPAssStart) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	m.SourceAddrLen = r.u16()
	m.SourceAddr = r.str(int(m.SourceAddrLen))
	return r.err
}

// UDPAssRes

func (m *UDPAssRes) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	w.putU16(m.OK)
	w.putU16(m.AddrLen)
	w.putStr(m.Addr)
	return w.Bytes()
}

func (m *UDPAssRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	m.OK = r.u16()
	m.AddrLen = r.u16()
	m.Addr = r.str(int(m.AddrLen))
	return r.err
}

// SocksTCPFin

func (m *SocksTCPFin) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	return w.Bytes()
}

func (m *SocksTCPFin) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	return r.err
}

// SocksReady

func (m *SocksReady) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *SocksReady) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// ForwardTest

func (m *ForwardTest) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.AddrLen)
	w.putStr(m.Addr)
	return w.Bytes()
}

func (m *ForwardTest) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.AddrLen = r.u16()
	m.Addr = r.str(int(m.AddrLen))
	return r.err
}

// ForwardStart

func (m *ForwardStart) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	w.putU16(m.AddrLen)
	w.putStr(m.Addr)
	return w.Bytes()
}

func (m *ForwardStart) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	m.AddrLen = r.u16()
	m.Addr = r.str(int(m.AddrLen))
	return r.err
}

// ForwardReady

func (m *ForwardReady) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *ForwardReady) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// ForwardData

func (m *ForwardData) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	w.putU64(m.DataLen)
	w.putBytes(m.Data)
	return w.Bytes()
}

func (m *ForwardData) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	m.DataLen = r.u64()
	m.Data = r.readBytes(int(m.DataLen))
	return r.err
}

// ForwardFin

func (m *ForwardFin) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	return w.Bytes()
}

func (m *ForwardFin) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	return r.err
}

// BackwardTest

func (m *BackwardTest) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.LPortLen)
	w.putStr(m.LPort)
	w.putU16(m.RPortLen)
	w.putStr(m.RPort)
	return w.Bytes()
}

func (m *BackwardTest) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.LPortLen = r.u16()
	m.LPort = r.str(int(m.LPortLen))
	m.RPortLen = r.u16()
	m.RPort = r.str(int(m.RPortLen))
	return r.err
}

// BackwardStart

func (m *BackwardStart) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.UUIDLen)
	w.putStr(m.UUID)
	w.putU16(m.LPortLen)
	w.putStr(m.LPort)
	w.putU16(m.RPortLen)
	w.putStr(m.RPort)
	return w.Bytes()
}

func (m *BackwardStart) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.UUIDLen = r.u16()
	m.UUID = r.str(int(m.UUIDLen))
	m.LPortLen = r.u16()
	m.LPort = r.str(int(m.LPortLen))
	m.RPortLen = r.u16()
	m.RPort = r.str(int(m.RPortLen))
	return r.err
}

// BackwardReady

func (m *BackwardReady) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *BackwardReady) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// BackwardSeq

func (m *BackwardSeq) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	w.putU16(m.RPortLen)
	w.putStr(m.RPort)
	return w.Bytes()
}

func (m *BackwardSeq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	m.RPortLen = r.u16()
	m.RPort = r.str(int(m.RPortLen))
	return r.err
}

// BackwardData

func (m *BackwardData) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	w.putU64(m.DataLen)
	w.putBytes(m.Data)
	return w.Bytes()
}

func (m *BackwardData) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	m.DataLen = r.u64()
	m.Data = r.readBytes(int(m.DataLen))
	return r.err
}

// BackWardFin

func (m *BackWardFin) MarshalBinary() []byte {
	var w binWriter
	w.putU64(m.Seq)
	return w.Bytes()
}

func (m *BackWardFin) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Seq = r.u64()
	return r.err
}

// BackwardStop

func (m *BackwardStop) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.All)
	w.putU16(m.RPortLen)
	w.putStr(m.RPort)
	return w.Bytes()
}

func (m *BackwardStop) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.All = r.u16()
	m.RPortLen = r.u16()
	m.RPort = r.str(int(m.RPortLen))
	return r.err
}

// BackwardStopDone

func (m *BackwardStopDone) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.All)
	w.putU16(m.UUIDLen)
	w.putStr(m.UUID)
	w.putU16(m.RPortLen)
	w.putStr(m.RPort)
	return w.Bytes()
}

func (m *BackwardStopDone) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.All = r.u16()
	m.UUIDLen = r.u16()
	m.UUID = r.str(int(m.UUIDLen))
	m.RPortLen = r.u16()
	m.RPort = r.str(int(m.RPortLen))
	return r.err
}

// ConnectStart

func (m *ConnectStart) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.AddrLen)
	w.putStr(m.Addr)
	return w.Bytes()
}

func (m *ConnectStart) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.AddrLen = r.u16()
	m.Addr = r.str(int(m.AddrLen))
	return r.err
}

// ConnectDone

func (m *ConnectDone) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *ConnectDone) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// NodeOffline

func (m *NodeOffline) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.UUIDLen)
	w.putStr(m.UUID)
	return w.Bytes()
}

func (m *NodeOffline) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.UUIDLen = r.u16()
	m.UUID = r.str(int(m.UUIDLen))
	return r.err
}

// NodeReonline

func (m *NodeReonline) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.ParentUUIDLen)
	w.putStr(m.ParentUUID)
	w.putU16(m.UUIDLen)
	w.putStr(m.UUID)
	w.putU16(m.IPLen)
	w.putStr(m.IP)
	return w.Bytes()
}

func (m *NodeReonline) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.ParentUUIDLen = r.u16()
	m.ParentUUID = r.str(int(m.ParentUUIDLen))
	m.UUIDLen = r.u16()
	m.UUID = r.str(int(m.UUIDLen))
	m.IPLen = r.u16()
	m.IP = r.str(int(m.IPLen))
	return r.err
}

// UpstreamOffline

func (m *UpstreamOffline) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *UpstreamOffline) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// UpstreamReonline

func (m *UpstreamReonline) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *UpstreamReonline) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// Shutdown

func (m *Shutdown) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *Shutdown) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// HeartbeatMsg

func (m *HeartbeatMsg) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.Ping)
	return w.Bytes()
}

func (m *HeartbeatMsg) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Ping = r.u16()
	return r.err
}

// TransportSwitchReq

func (m *TransportSwitchReq) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.Method)
	w.putU16(m.AddrLen)
	w.putStr(m.Addr)
	return w.Bytes()
}

func (m *TransportSwitchReq) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.Method = r.u16()
	m.AddrLen = r.u16()
	m.Addr = r.str(int(m.AddrLen))
	return r.err
}

// TransportSwitchRes

func (m *TransportSwitchRes) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	w.putU16(m.AddrLen)
	w.putStr(m.Addr)
	return w.Bytes()
}

func (m *TransportSwitchRes) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	m.AddrLen = r.u16()
	m.Addr = r.str(int(m.AddrLen))
	return r.err
}

// TransportSwitchDone

func (m *TransportSwitchDone) MarshalBinary() []byte {
	var w binWriter
	w.putU16(m.OK)
	return w.Bytes()
}

func (m *TransportSwitchDone) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.OK = r.u16()
	return r.err
}

// RouteTableMsg

func (m *RouteTableMsg) MarshalBinary() []byte {
	var w binWriter
	w.putU32(uint32(len(m.Entries)))
	w.putBytes([]byte(m.Entries))
	return w.Bytes()
}

func (m *RouteTableMsg) UnmarshalBinary(data []byte) error {
	r := binReader{data: data}
	m.EntriesLen = r.u32()
	m.Entries = r.str(int(m.EntriesLen))
	return r.err
}
