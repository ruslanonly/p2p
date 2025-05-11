package classifier

import (
	"log"
	"threats/internal/classifier/model"

	onnx "github.com/yalue/onnxruntime_go"
)

type Classifier struct {
}

func NewClassifier() *Classifier {
	err := onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("failed to initialize environment: %v", err)
	}

	return &Classifier{}
}

func (c *Classifier) Classify(vector []float32) model.TrafficClass {
	inputShape := onnx.Shape{1, 41}
	inputTensor, err := onnx.NewTensor(inputShape, vector)
	if err != nil {
		log.Fatalf("failed to create input tensor: %v", err)
	}

	outputShape := onnx.Shape{1, 2}
	outputTensor, err := onnx.NewEmptyTensor[float32](outputShape)
	if err != nil {
		log.Fatalf("failed to create output tensor: %v", err)
	}

	inputNames := []string{"input"}
	outputNames := []string{"probabilities"}

	session, err := onnx.NewSession("model.onnx", inputNames, outputNames, []*onnx.Tensor[float32]{inputTensor}, []*onnx.Tensor[float32]{outputTensor})
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	defer session.Destroy()

	err = session.Run()
	if err != nil {
		log.Fatalf("failed to run inference: %v", err)
	}

	probs := outputTensor.GetData()

	if probs[1] > probs[0] {
		return model.RedTrafficClass
	}

	return model.GreenTrafficClass
}
