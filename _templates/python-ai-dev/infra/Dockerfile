FROM pytorch/pytorch:2.4.0-cuda11.8-cudnn9-runtime AS base

# Update and install common tools
RUN apt update && apt install -y \
    bash \
    curl \
    git \
    jq \
    openssh-server \
    python3 \
    rsync \
    sudo \
    unzip \
    wget \
    make \
    gcc \
    libffi-dev \
    libssl-dev \
    python3-dev \
    cargo \
    zsh \
    passwd \
 && apt clean && rm -rf /var/lib/apt/lists/*

# Create user 'jarvis' with home and bash shell
RUN useradd -m -s /bin/bash jarvis && \
    echo "jarvis:jarvis" | chpasswd && \
    usermod -aG sudo jarvis && \
    echo "jarvis ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

ENV HOME=/home/jarvis
USER jarvis
WORKDIR /home/jarvis

FROM base AS dev

# Use root to install oh-my-zsh and setup tools
USER root
RUN chsh -s /usr/bin/zsh jarvis

USER jarvis
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended

# Install Zsh plugins
RUN git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/zsh-autosuggestions && \
    git clone https://github.com/zsh-users/zsh-syntax-highlighting.git ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/zsh-syntax-highlighting && \
    sed -i 's/plugins=(git)/plugins=(git zsh-autosuggestions zsh-syntax-highlighting)/' $HOME/.zshrc

# Install Python tools
ENV HOME=/home/jarvis
ENV PATH=/home/jarvis/.local/bin:$PATH

RUN echo 'export PATH="$HOME/.local/bin:$PATH"' >> $HOME/.zshrc
RUN pip install --upgrade pip && pip install uv --break-system-packages

# Prepare shell history and app dir
RUN touch $HOME/.zsh_history && mkdir -p $HOME/apps

WORKDIR /home/jarvis/apps

COPY ../ /home/jarvis/apps


CMD ["zsh"]
