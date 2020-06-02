test-python2:
	docker run -v $$(pwd):/test -w /test --rm -ti python:2 ./test.sh

test-python3:
	docker run -v $$(pwd):/test -w /test --rm -ti python:3.6 ./test.sh

.PHONY: test
test: test-python2 test-python3
