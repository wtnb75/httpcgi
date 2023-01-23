FROM scratch
COPY httpcgi /
ENTRYPOINT [ "/httpcgi" ]
LABEL org.opencontainers.image.title=httpcgi
LABEL org.opencontainers.image.description="run cgi"
LABEL org.opencontainers.image.authors="Watanabe Takashi <wtnb75@gmail.com>"
LABEL org.opencontainers.image.url="https://github.com/wtnb75/httpcgi"
LABEL org.opencontainers.image.source="https://github.com/wtnb75/httpcgi"
LABEL org.opencontainers.image.licenses=MIT
