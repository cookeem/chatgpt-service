package chat

type Config struct {
	ApiKey          string `yaml:"apiKey" json:"apiKey" bson:"apiKey" validate:"required"`
	Port            int    `yaml:"port" json:"port" bson:"port" validate:"required"`
	IntervalSeconds int    `yaml:"intervalSeconds" json:"intervalSeconds" bson:"intervalSeconds" validate:"required"`
	Model           string `yaml:"model" json:"model" bson:"model" validate:"required"`
	MaxLength       int    `yaml:"maxLength" json:"maxLength" bson:"maxLength" validate:"required"`
	Cors            bool   `yaml:"cors" json:"cors" bson:"cors" validate:""`
}
