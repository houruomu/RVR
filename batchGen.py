infile = open("serverlist.txt", "r")
outfile = open("control_gen.bat", "w")
template_acc = "echo y | plink.exe -ssh houruomu@{}.comp.nus.edu.sg -pw ***REMOVED*** \"exit\"\n"
template_control = "plink.exe -ssh houruomu@{}.comp.nus.edu.sg -pw ***REMOVED*** -m command.txt > output.txt\n"
for line in infile.readlines():
    line = line.strip()
    outfile.write(template_control.format(line))