FROM busybox

COPY ./myhttpd /myhttpd

CMD ["/myhttpd"]
