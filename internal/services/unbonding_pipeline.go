package services

type UnbondingPipeline struct {
}

func NewUnbondingPipeline() *UnbondingPipeline {
	return &UnbondingPipeline{}
}

func (up *UnbondingPipeline) Run() error {
	return nil
}
