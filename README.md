# Brutel 

## telnet auto login tool

To use it without installation, you have to rename config.example.ini to 
config.ini and change the credentials.

Then use it in a very simple and straight-forward way:
`./brutal 1.1.1.1`
where 1.1.1.1 is your IP or domain name

For Windows, use .exe build:
`.\brutal.exe 1.1.1.1`

If you want your system to open your `telnet://` links by clicking on it, make 
install

# Windows installation

1. First rename config.example.ini to config.ini and change the credentials
2. Rename one if your binaries to `brutel.exe` and locate it somewhere (or not)
3. Edit path at `install_win.reg` file
4. Run `install_win.reg`

# Linux installation

1. First rename config.example.ini to config.ini and change the credentials
2. Then rename one of your binaries to `brutel` and locate it somewhere (or not)
3. Edit path at `brutel.sh` file and make sure it's executable 
4. Edit Exec and Path variables at file brutel.desktop and move it to your HOME 
~/.local/share/applications directory 
5. Run `xdg-mime default brutel.desktop x-scheme-handler/telnet`
6. If you want to make sure you're doing right run 
`xdg-mime query default x-scheme-handler/telnet` and you should see 
`brutel.desktop`

To test your installation check test.html file: replace 1.1.1.1 to your telnet 
IP or domain and open it in a browser.

# License 

MIT license, do WTF u want ;)
