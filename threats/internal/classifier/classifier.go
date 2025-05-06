package classifier

import (
	"threats/internal/classifier/model"
)

type Classifier struct {
}

func NewClassifier() *Classifier {
	return &Classifier{}
}

func (c *Classifier) Classify(p model.TCPIPClassificationParameters) model.TrafficClass {
	return model.GreenTrafficClass
}
