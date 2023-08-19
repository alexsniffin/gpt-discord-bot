# gpt-discord-bot

A Discord bot for having conversions with users utilizing GPT through OpenAI or Private language models.

## Features
- /chat command 
  - `dialogue_prompt` argument  will set the system prompt for the bot
  - will create a new Thread where you can start the conversion
- language prompting uses OpenAI's Completion API
  - Default model is `gpt-3.5-turbo` but can be switched using the `MODEL` env
  - The host can be switched using the `OPENAI_API_BASE` env to a private API

## Config
| Environment Variable            | Description                                                | Default Value                              | Required |
|---------------------------------|------------------------------------------------------------|--------------------------------------------|----------|
| `MODEL`                         | Model name for OpenAI.                                     | `gpt-3.5-turbo`                            | No       |
| `OPENAI_API_KEY`                | OpenAI API key.                                            | -                                          | Yes      |
| `OPENAI_API_BASE`               | Base URL for the OpenAI API.                               | -                                          | Yes      |
| `DISCORD_BOT_TOKEN`             | Token for the Discord bot.                                 | -                                          | Yes      |
| `DISCORD_APPLICATION_ID`        | The application ID in Discord.                             | -                                          | Yes      |
| `THREAD_NAME`                   | Name of the thread in the chat.                            | `GPT Chat Thread`                          | No       |
| `SECONDS_DELAY_RECEIVE_MESSAGE` | Delay (in seconds) before the bot receives a message.      | `1`                                        | No       |
| `MAX_MESSAGES_LENGTH`           | Maximum length of the messages the bot can handle.         | `100`                                      | No       |
| `SYSTEM_PROMPT_SUFFIX`          | Suffix for the system prompt. Can contain a format string. | `Your name is %s, reply back to the user.` | No       |