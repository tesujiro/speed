# speed
Simple performance monitor command for reading Linux file.

## Description

## Requirement

## Usage
### Show speed of Stdin
SomeCommand | speed | SomeNextCommand

### Limit bandwidth
SomeCommand | speed -bandwidth 10MB | SomeNextCommand

### Show progress
speed -graph SomeBigFile | SomeNextCommand

### Show progress widh bandwidth
speed -graph SomeBigFile -graph | SomeNextCommand

## VS. 
pv: 

## Install
go get github.com/tesujiro/speed

## Licence

[MIT](https://github.com/tcnksm/tool/blob/master/LICENCE)

## Author

[tesujiro](https://github.com/tesujiro)

