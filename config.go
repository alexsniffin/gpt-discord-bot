package main

type config struct {
	Model                      string `env:"MODEL" envDefault:"gpt-3.5-turbo"`
	OpenaiAPIKey               string `env:"OPENAI_API_KEY,required"`
	OpenaiAPIBase              string `env:"OPENAI_API_BASE,required"`
	ApplicationID              string `env:"DISCORD_APPLICATION_ID,required"`
	DiscordBotToken            string `env:"DISCORD_BOT_TOKEN,required"`
	ThreadName                 string `env:"THREAD_NAME" envDefault:"GPT Chat Thread"`
	SecondsDelayReceiveMessage int    `env:"SECONDS_DELAY_RECEIVE_MESSAGE" envDefault:"1"`
	MaxMessagesLength          int    `env:"MAX_MESSAGES_LENGTH" envDefault:"100"`
	SystemPromptSuffix         string `env:"SYSTEM_PROMPT_SUFFIX" envDefault:"Your name is %s, reply back to the user."`
}
