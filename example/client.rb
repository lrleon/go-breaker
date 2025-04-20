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

def post_json(endpoint_name, payload)
  url = "#{$url}#{endpoint_name}"
  puts "Posting to #{url} with payload: #{payload.to_json}"
  response = HTTP.headers('Content-Type' => 'application/json').post(url, json: payload)
  JSON.parse(response.body)
end

# make a request to the server
def set_delay(delay)
  post_json('set_delay', { delay: delay })
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
  post_json('set_memory', { threshold: memory })
end

def set_latency(threshold)
  post_json('set_latency', { threshold: threshold })
end

def set_latency_window_size(window_size)
  post_json('set_latency_window_size', { size: window_size })
end

def set_percentile(percentile)
  post_json('set_percentile', { percentile: percentile })
end

def set_wait_time(wait_time)
  post_json('set_wait', { wait_time: wait_time })
end

def get_latencies(threshold); consult("latencies_above_threshold/#{threshold}"); end

def reset; post_json('reset', {}); end
def enable; post_json('enable', {}); end
def disable; post_json('disable', {}); end
