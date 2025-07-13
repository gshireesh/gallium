FROM alpine AS base

RUN apk update && apk add --no-cache \
    bash \
    curl \
    docker \
    git \
    jq \
    openssh \
    python3 \
    py3-pip \
    rsync \
    sudo \
    unzip \
    wget
#-------------------------------
#       To add user
#-------------------------------
RUN apk add sudo
# Add user named jarvis
# apline equalant of  adduser -hs /bin/bash jarvis
RUN adduser -D -s /bin/bash jarvis
RUN echo "jarvis:jarvis" | chpasswd
RUN adduser jarvis wheel
RUN echo "jarvis ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

RUN apk add go make npm nodejs
RUN npm install -g @bufbuild/buf nodemon pm2

#-------------------------------
USER jarvis
ENV HOME /home/jarvis

FROM base AS dev
USER root
RUN apk add zsh shadow
USER jarvis
RUN  sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended
# Set Zsh as the default shell
RUN chsh -s $(which zsh)
# add autocompletion zsh
RUN git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions
# Install Zsh syntax highlighting plugin
RUN git clone https://github.com/zsh-users/zsh-syntax-highlighting.git ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-syntax-highlighting

# Enable the plugins in .zshrc
RUN sed -i 's/plugins=(git)/plugins=(git zsh-autosuggestions zsh-syntax-highlighting)/' ~/.zshrc


RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
#add terraform and terragrunt
USER root
#RUN wget https://releases.hashicorp.com/terraform/1.9.4/terraform_1.9.4_linux_amd64.zip
#RUN unzip terraform_1.9.4_linux_amd64.zip
#RUN sudo mv terraform /usr/local/bin/
#RUN #rm terraform_1.9.4_linux_amd64.zip
RUN wget https://github.com/gruntwork-io/terragrunt/releases/download/v0.66.3/terragrunt_linux_amd64
RUN mv terragrunt_linux_amd64 /usr/local/bin/terragrunt
RUN chmod +x /usr/local/bin/terragrunt
RUN chown jarvis:jarvis /usr/local/bin/terragrunt
RUN apk add --no-cache gcc musl-dev libffi-dev openssl-dev python3-dev cargo poetry

USER jarvis
RUN curl -L https://raw.githubusercontent.com/warrensbox/terraform-switcher/master/install.sh | bash -s -- -b $HOME/.local/bin
RUN $HOME/.local/bin/tfswitch --latest -b ~/.local/bin/terraform
RUN echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.zshrc
ENV PATH=$PATH:/usr/local/go/bin:/home/jarvis/.local/bin
WORKDIR /home/jarvis/apps
CMD ["su", "-", "jarvis", "-c", "/bin/sh"]

