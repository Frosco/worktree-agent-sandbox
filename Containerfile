FROM node:24-bookworm-slim

# Install basic tools
RUN apt-get update && apt-get install -y \
    git \
    curl \
    ripgrep \
    fd-find \
    && rm -rf /var/lib/apt/lists/*

# Install mise
RUN curl https://mise.run | sh
ENV PATH="/root/.local/bin:$PATH"

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Use the existing node user (UID/GID 1000) from the base image
USER node
WORKDIR /home/node

# Mise for non-root user
RUN curl https://mise.run | sh
ENV PATH="/home/node/.local/bin:$PATH"

# Entry script will be bind-mounted
CMD ["bash"]
