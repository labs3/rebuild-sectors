package worker

import (
	"batch_rebuild/services"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

type IMessage interface {
	ID() uint8
	Name() string
	Encode() ([]byte, error)
	Decode(data []byte) error
}

const (
	AckType uint8 = iota
	TaskMsgType
)

type AckMsg struct {
	body string
}

func NewAckMsg() *AckMsg {
	return &AckMsg{
		body: "OK",
	}
}

func (p *AckMsg) ID() uint8 {
	return AckType
}

func (p *AckMsg) Name() string {
	return "AckMsg"
}

func (p *AckMsg) Encode() ([]byte, error) {
	return []byte(p.body), nil
}

func (p *AckMsg) Decode(data []byte) error {
	p.body = string(data)
	return nil
}

type TaskMsg struct {
	WTask *services.WorkerTask
}

func NewTaskMsg(task *services.WorkerTask) *TaskMsg {
	return &TaskMsg{WTask: task}
}

func (t *TaskMsg) ID() uint8 {
	return TaskMsgType
}

func (t *TaskMsg) Name() string {
	return "TaskMsg"
}

func (t *TaskMsg) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *TaskMsg) Decode(data []byte) error {
	return json.Unmarshal(data, t.WTask)
}

func PackMsg(msg IMessage) ([]byte, error) {
	msgBody, err := msg.Encode()
	if err != nil {
		return nil, err
	}
	msgLen := 4 + 1 + len(msgBody) // = 总长度字节数4 + 消息ID字节数1 + 消息体字节数（uint16, 即最大长度65535）
	buf := make([]byte, msgLen)

	// 前4个byte放置总长度
	binary.BigEndian.PutUint16(buf[:4], uint16(msgLen))
	buf[4] = msg.ID()
	copy(buf[5:], msgBody)

	return buf, nil
}

func UnpackMsg(data []byte) (m IMessage, err error) {

	msgLen := binary.BigEndian.Uint16(data[:4])
	msgID := data[4]
	msgBody := data[5 : msgLen-5]

	switch uint8(msgID) {
	case AckType:
		ackMsg := &AckMsg{}
		ackMsg.Decode(msgBody)
		m = ackMsg
	case TaskMsgType:
		taskMsg := &TaskMsg{}
		err = taskMsg.Decode(msgBody)
		if err != nil {
			return
		}
		m = taskMsg
	default:
		return nil, fmt.Errorf("unknown msg ID %d", msgID)
	}

	return
}
