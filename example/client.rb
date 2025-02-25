# simple client for testing go go-breaker package

require 'json'
require 'http'
require 'byebug'


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

def latency
  consult('latency')
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

def memory_usage; consult('memory_usage'); end

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

def get_latencies(threshold); consult("latencies_above_threshold/#{threshold}"); end
