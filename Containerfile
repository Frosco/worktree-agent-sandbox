FROM node:20-bookworm-slim

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

# Create non-root user matching typical host UID
ARG USER_ID=1000
ARG GROUP_ID=1000
RUN groupadd -g $GROUP_ID appuser && \
    useradd -m -u $USER_ID -g $GROUP_ID appuser

USER appuser
WORKDIR /home/appuser

# Mise for non-root user
RUN curl https://mise.run | sh
ENV PATH="/home/appuser/.local/bin:$PATH"

# Entry script will be bind-mounted
CMD ["bash"]
