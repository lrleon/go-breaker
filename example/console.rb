# give a console on ruby for calling the client function

require_relative 'client'

require 'irb'

ARGV.clear # otherwise, IRB will try to parse the arguments

IRB.start(__FILE__)
