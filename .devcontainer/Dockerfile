FROM golang:1.24

# Set environment variables
ENV DEBIAN_FRONTEND=noninteractive
ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

# Install development tools and utilities
RUN apt-get update && apt-get install -y \
    git \
    git-lfs \
    curl \
    wget \
    vim \
    tmux \
    zsh \
    python3 \
    python3-pip \
    python3-venv \
    locales \
    lsof \
    htop \
    tree \
    silversearcher-ag \
    cloc \
    unzip \
    ninja-build \
    gdb \
    pkg-config \
    libssl-dev \
    bear \
    ccache \
    openssh-server \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Set up locale
RUN locale-gen en_US.UTF-8

# Install Go tools
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install golang.org/x/tools/gopls@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    go install honnef.co/go/tools/cmd/staticcheck@latest

# Install Python tools
RUN python3 -m pip install --no-cache-dir \
    pytest \
    black \
    isort \
    flake8 \
    mypy \
    pylint \
    pre-commit

# Install diff-so-fancy
RUN curl -LSso /usr/local/bin/diff-so-fancy https://github.com/so-fancy/diff-so-fancy/releases/download/v1.4.4/diff-so-fancy \
    && chmod +x /usr/local/bin/diff-so-fancy

# Install oh-my-zsh and plugins
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended \
    && git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions \
    && git clone https://github.com/zsh-users/zsh-syntax-highlighting.git ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-syntax-highlighting

# Configure Vim
COPY --chown=root:root <<-"EOF" /root/.vimrc
" Basic settings
set number
syntax on
set mouse=a
filetype indent on

" Indentation
set autoindent nosmartindent
set smarttab
set expandtab
set shiftwidth=4
set softtabstop=4

" Visual guides
set colorcolumn=120
highlight ColorColumn ctermbg=5

" Status line
set laststatus=2
set statusline=%<%f\ %h%m%r%=%{\"[\".(&fenc==\"\"?&enc:&fenc).((exists(\"+bomb\")\ &&\ &bomb)?\",B\":\"\").\"]\\ \"}%k\ %-14.(%l,%c%V%)\ %P

" Backspace behavior
set backspace=2

" Encoding
set encoding=utf-8
set fileencoding=utf-8
EOF

# Configure tmux
COPY --chown=root:root <<-"EOF" /root/.tmux.conf
# Pane border styling
set -g pane-border-style fg='#742727',bg=black
set -g pane-active-border-style fg=red,bg=black

# Status bar styling
set -g status-style bg='#0C8A92',fg=black

# Change prefix key to backtick
set-option -g prefix `
unbind C-b
bind-key ` send-prefix

# Split panes using - and = with current path
unbind '"'
bind - splitw -v -c '#{pane_current_path}'
unbind '%'
bind = splitw -h -c '#{pane_current_path}'

# Vi mode settings
set-window-option -g mode-keys vi

# Other settings
set-option -g escape-time 0
set-option -g base-index 1
set-window-option -g mouse on
EOF

# Configure Git
RUN git config --global core.editor "vim" \
    && git config --global core.whitespace "fix,-indent-with-non-tab,trailing-space,cr-at-eol" \
    && git config --global core.pager "diff-so-fancy | less --tabs=4 -RFX" \
    && git config --global color.ui true \
    && git config --global alias.lg "log --color --graph --pretty=format:'%Cred%h%Creset - %s %Cgreen(%cr) %C(bold blue)<%an>%Creset%C(auto)%d%Creset' --abbrev-commit --" \
    && git config --global pull.rebase true

# Configure zsh
COPY --chown=root:root <<-"EOF" /root/.zshrc
export ZSH="/root/.oh-my-zsh"

# Theme
ZSH_THEME="robbyrussell"

# Plugins
plugins=(
    git
    z
    golang
    python
    docker
    kubectl
    zsh-autosuggestions
    zsh-syntax-highlighting
)

source $ZSH/oh-my-zsh.sh

# Aliases
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'
alias vi='vim'

# Go paths
export GOPATH=/go
export PATH=$GOPATH/bin:$PATH

# Enhanced history
HISTSIZE=10000
SAVEHIST=10000
setopt HIST_IGNORE_ALL_DUPS
setopt HIST_FIND_NO_DUPS
setopt INC_APPEND_HISTORY
EOF

# Create a non-root user (optional, VS Code can be configured to use root)
RUN useradd -m -s /bin/zsh -u 1000 devuser \
    && mkdir -p /home/devuser/.config \
    && chown -R devuser:devuser /home/devuser \
    && cp /root/.vimrc /home/devuser/ \
    && cp /root/.tmux.conf /home/devuser/ \
    && cp -r /root/.oh-my-zsh /home/devuser/ \
    && cp /root/.zshrc /home/devuser/ \
    && chown -R devuser:devuser /home/devuser

# Set up configuration for devuser
RUN sed -i 's/\/root\/.oh-my-zsh/\/home\/devuser\/.oh-my-zsh/g' /home/devuser/.zshrc

# Set workspace directory
WORKDIR /workspace
