$cnt = 0
while ($true)
{
    write-output ("STDOUT $cnt")
    # write-output ("stderr $cnt") 1>&2
    sleep 1
    $cnt = $cnt + 1
}
