package classifier

import (
	"fmt"
	"log"
	"threats/internal/classifier/model"

	onnx "github.com/yalue/onnxruntime_go"
)

type Classifier struct {
}

func NewClassifier() *Classifier {
	return &Classifier{}
}

func (c *Classifier) Classify(p model.ClassificationParameters) model.TrafficClass {
	err := onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("failed to initialize environment: %v", err)
	}
	defer onnx.DestroyEnvironment()

	inputVector := make([]float32, 120)
	inputShape := onnx.Shape{1, 120}
	inputTensor, err := onnx.NewTensor(inputShape, inputVector)
	if err != nil {
		log.Fatalf("failed to create input tensor: %v", err)
	}

	outputShape := onnx.Shape{1, 2}
	outputTensor, err := onnx.NewEmptyTensor[float32](outputShape)
	if err != nil {
		log.Fatalf("failed to create output tensor: %v", err)
	}

	inputNames := []string{"input"}
	outputNames := []string{"output"}

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
	fmt.Println("Вероятности классов:", probs)

	if probs[1] > probs[0] {
		return model.RedTrafficClass
	}

	return model.GreenTrafficClass
}
