# simple client for testing go go-breaker package


require 'awesome_print'
require 'json'
require 'http'
require 'byebug'

# cli parameters are the host and port of the server. If not provided, default to localhost:8080

host = ARGV[0] || 'localhost'
port = ARGV[1] || 8080

url = "http://#{host}:#{port}/"

# make a request to the server
