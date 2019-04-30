FROM fedora:29

COPY ./bin/aws-s3-provisioner /usr/bin/

ENTRYPOINT ["/usr/bin/aws-s3-provisioner"]
CMD ["-v=2", "-alsologtostderr"]