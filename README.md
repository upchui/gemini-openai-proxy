# Gemini-OpenAI-Proxy

This project is a **fork** of the original Gemini-OpenAI-Proxy, specifically enhanced to support **all Google Gemini models**. It allows seamless integration with applications designed for the OpenAI API by bridging them to the Google Gemini protocol.

## Features

- **Full Google Gemini Model Support**: Access all Gemini models.
- **Compatibility with OpenAI API Requests**: Supports Chat Completions, Embeddings, and other endpoints.
- **Simple Deployment**: Get started with a single command.
- **Automatic Model Mapping**: OpenAI model names are automatically mapped to equivalent Gemini models.

### Improvement over the Original Project
The original project only supports a selected set of models that are hardcoded. This fork improves upon this by dynamically fetching the list of all available Gemini models from Google and making them readily accessible.

## Quick Start

1. **Clone the repository**:
    ```bash
    git clone https://github.com/zhu327/gemini-openai-proxy.git
    cd gemini-openai-proxy
    ```

2. **Start the proxy**:
    ```bash
    docker-compose up -d
    ```