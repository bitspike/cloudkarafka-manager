package kafka

import (
	"github.com/84codes/cloudkarafka-mgmt/store"

	"github.com/confluentinc/confluent-kafka-go/kafka"

	"bytes"
	"encoding/binary"
	"errors"
	"strconv"
)

func consumerOffsetsMessage(msg *kafka.Message) {
	var keyver, valver uint16
	var group, topic string
	var partition uint32
	var offset, timestamp uint64

	buf := bytes.NewBuffer(msg.Key)
	err := binary.Read(buf, binary.BigEndian, &keyver)
	switch keyver {
	case 0, 1:
		group, err = readString(buf)
		if err != nil {
			return //m, errors.New(fmt.Sprintf("Failed to decode %s:%v offset %v: group\n", msg.Topic, msg.Partition, msg.Offset))
		}
		topic, err = readString(buf)
		if err != nil {
			return //m, errors.New(fmt.Sprintf("Failed to decode %s:%v offset %v: topic\n", msg.Topic, msg.Partition, msg.Offset))
		}
		err = binary.Read(buf, binary.BigEndian, &partition)
		if err != nil {
			return //m, errors.New(fmt.Sprintf("Failed to decode %s:%v offset %v: partition\n", msg.Topic, msg.Partition, msg.Offset))
		}
	case 2:
		//This is a message when a consumer starts/stop consuming from a topic
		return //m, errors.New(fmt.Sprintf("Discarding group metadata message with key version 2\n"))
	default:
		return //m, errors.New(fmt.Sprintf("Failed to decode %s:%v offset %v: keyver %v\n", msg.Topic, msg.Partition, msg.Offset, keyver))
	}

	buf = bytes.NewBuffer(msg.Value)
	err = binary.Read(buf, binary.BigEndian, &valver)
	if (err != nil) || ((valver != 0) && (valver != 1)) {
		return //m, errors.New(fmt.Sprintf("Failed to decode %s:%v offset %v: valver %v\n", msg.Topic, msg.Partition, msg.Offset, valver))
	}
	err = binary.Read(buf, binary.BigEndian, &offset)
	if err != nil {
		return //m, errors.New(fmt.Sprintf("Failed to decode %s:%v offset %v: offset\n", msg.Topic, msg.Partition, msg.Offset))
	}
	_, err = readString(buf)
	if err != nil {
		return //m, errors.New(fmt.Sprintf("Failed to decode %s:%v offset %v: metadata\n", msg.Topic, msg.Partition, msg.Offset))
	}
	err = binary.Read(buf, binary.BigEndian, &timestamp)
	if err != nil {
		return //m, errors.New(fmt.Sprintf("Failed to decode %s:%v offset %v: timestamp\n", msg.Topic, msg.Partition, msg.Offset))
	}
	if topic == "__consumer_offsets" || topic == "__cloudkarafka_metrics" {
		return
	}
	p := strconv.Itoa(int(partition))
	store.Put("consumer", int(offset), int64(timestamp), group, topic, p)
}

func readString(buf *bytes.Buffer) (string, error) {
	var strlen uint16
	err := binary.Read(buf, binary.BigEndian, &strlen)
	if err != nil {
		return "", err
	}
	strbytes := make([]byte, strlen)
	n, err := buf.Read(strbytes)
	if (err != nil) || (n != int(strlen)) {
		return "", errors.New("string underflow")
	}
	return string(strbytes), nil
}
