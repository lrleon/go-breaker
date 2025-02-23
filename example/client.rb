# simple client for testing go go-breaker package

require 'awesome_print'
require 'json'
require 'http'
require 'byebug'

# Cli parameters are the host and port of the server. If not provided, default to localhost:8080

host = ARGV[0] || 'localhost'
port = ARGV[1] || 8080

$url = "http://#{host}:#{port}/"

def consult(endpoint_name)
  url = "#{$url}#{endpoint_name}"
  puts "Requesting #{url}"
  response = HTTP.get(url)
  JSON.parse(response.body)
end

def set(endpoint_name, value)
  url = "#{$url}#{endpoint_name}/#{value}"
  puts "Requesting #{url}"
  response = HTTP.get(url)
  JSON.parse(response.body)
end

# make a request to the server
def set_delay(delay)
  set('set_delay', delay)
end

def request
  consult('test')
end

def memory
  consult('memory')
end

def size
  consult('size')
end

def latency_window_size
  consult('latency_window_size')
end

def percentile
  consult('percentile')
end

def wait_time
  consult('wait')
end

def set_memory(memory)
  set('set_memory', memory)
end

def set_latency(size)
  set('set_latency', size)
end

def set_latency_window_size(window_size)
  set('set_latency_window_size', window_size)
end

def set_percentile(percentile)
  set('set_percentile', percentile)
end

def set_wait_time(wait_time)
  set('set_wait', wait_time)
end
