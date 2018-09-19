Randomized View Reconciliation
==============================
Usage:
`
go build  
./RVR [--mode=controller|spawner|node] [--server=CONTROLLER_ADDRESS]
`
The controller supports the following commands:  
batch : Automated batch testing (accroding to the scheme written in algorithm/Controller.batch)  
state : pick a random node and report its state  
measure : collect the average ping data across the nodes  
setup : setup nodes accroding to the Default Parameters  
start : start the view reconciliation on all the nodes simultaneously  
reset : kill all the nodes  
spawn : create a node at a randomly selected spwaner server  
report : collect state information from the nodes, and form a report of the overall state of the protocol  
exit  : let all the nodes, spawners exit, then the program exits  
