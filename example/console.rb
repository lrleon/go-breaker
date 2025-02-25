# give a console on ruby for calling the client function

require_relative 'client'

# Cli parameters are the host and port of the server. If not provided, default to localhost:8080

host = ARGV[0] || 'localhost'
port = ARGV[1] || 8080

$url = "http://#{host}:#{port}/breaker/"

require 'irb'

ARGV.clear # otherwise, IRB will try to parse the arguments

IRB.start(__FILE__)
