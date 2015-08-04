FROM scratch
ADD bin/lipwig /lipwig
EXPOSE 8787
ENTRYPOINT ["/lipwig"]
