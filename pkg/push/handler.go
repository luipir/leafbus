package push

import (
	"log"
	"time"

	"github.com/brutella/can"
	"github.com/prometheus/prometheus/pkg/labels"
)

const (
	name = "__name__"
)

type Handler struct {
	cortex *cortex
}

func NewHandler(address string) (*Handler, error) {
	c, err := newCortex(address)
	if err != nil {
		return nil, err
	}
	h := &Handler{
		cortex: c,
	}
	return h, nil
}

func (h *Handler) Handle(frame can.Frame) {

	switch frame.ID {
	case 0x55B:
		//SOC
		if len(h.cortex.data) >= 100 {
			log.Println("Ignoring packet, send buffer is full")
			return
		}
		currCharge := (uint16(frame.Data[0]) << 2) | (uint16(frame.Data[1]) >> 6)
		h.SendMetric("soc", nil, time.Now(), float64(currCharge))

	case 0x1DA:
		//Battery Current and Voltage
		if len(h.cortex.data) >= 100 {
			log.Println("Ignoring packet, send buffer is full")
			return
		}
		var motorAmps int16
		if frame.Data[2]&0b00000100 == 0b00000100 {
			motorAmps = int16(((uint16(frame.Data[2]&0b00000111) << 8) | 0b1111100000000000) | uint16(frame.Data[3]))
		} else {
			motorAmps = int16(((uint16(frame.Data[2]&0b00000111) << 8) & 0b0000011111111111) | uint16(frame.Data[3]))
		}
		motorSpeed := int16(uint16(frame.Data[4])<<8 | uint16(frame.Data[5]))
		ts := time.Now()
		h.SendMetric("motor_amps", nil, ts, float64(motorAmps))
		h.SendMetric("motor_rpm", nil, ts, float64(motorSpeed))

	case 0x1DB:
		//Battery Current and Voltage
		if len(h.cortex.data) >= 100 {
			log.Println("Ignoring packet, send buffer is full")
			return
		}
		// Even though the doc says the LSB for current is 0.5 it seems to reflect the actual charger current
		// more accurately when I don't ignore the last bit
		var battCurrent int16
		if frame.Data[0]&0b10000000 == 0b10000000 {
			battCurrent = int16((uint16(frame.Data[0]) << 3) | 0b1111100000000000 | uint16(frame.Data[1]>>6))
		} else {
			battCurrent = int16((uint16(frame.Data[0])<<3)&0b0000011111111111 | uint16(frame.Data[1]>>6))
		}
		// The voltage however seems to be more accurate when i do throw away the LSB (the doc would have us
		// shift left here 2 and add 3 from the second byte however that gave me 700+ volts)
		currVoltage := (uint16(frame.Data[2]) << 1) | (uint16(frame.Data[3]&0b11000000) >> 7)
		ts := time.Now()
		h.SendMetric("battery_amps", nil, ts, float64(battCurrent))
		h.SendMetric("battery_volts", nil, ts, float64(currVoltage))
	}
}

func (h *Handler) SendMetric(metricName string, additionalLabels labels.Labels, timestamp time.Time, val float64) {
	p := packetPool.Get().(*packet)
	ts := timestamp.UnixNano() / int64(time.Millisecond)
	p.sample.TimestampMs = ts
	p.sample.Value = val
	l := labelPool.Get().(labels.Label)
	l.Name = name
	l.Value = metricName
	p.labels = append(p.labels, l)
	if additionalLabels != nil {
		p.labels = append(p.labels, additionalLabels...)
	}
	h.cortex.data <- p
}
