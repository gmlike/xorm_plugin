package  xorm_plugin

import (
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"xorm.io/xorm"
)

//map for converting mysql type to golang types
var typeForMysqlToGo = map[string]string{
	"int":                "int64",
	"integer":            "int64",
	"tinyint":            "int64",
	"smallint":           "int64",
	"mediumint":          "int64",
	"bigint":             "int64",
	"int unsigned":       "int64",
	"integer unsigned":   "int64",
	"tinyint unsigned":   "int64",
	"smallint unsigned":  "int64",
	"mediumint unsigned": "int64",
	"bigint unsigned":    "int64",
	"bit":                "int64",
	"bool":               "bool",
	"enum":               "string",
	"set":                "string",
	"varchar":            "string",
	"char":               "string",
	"tinytext":           "string",
	"mediumtext":         "string",
	"text":               "string",
	"longtext":           "string",
	"blob":               "string",
	"tinyblob":           "string",
	"mediumblob":         "string",
	"longblob":           "string",
	"date":               "time.Time",
	"datetime":           "time.Time",
	"timestamp":          "time.Time",
	"time":               "time.Time",
	"float":              "float64",
	"double":             "float64",
	"decimal":            "float64",
	"binary":             "string",
	"varbinary":          "string",
}

//func main() {
//   //path, err := os.Executable()
//   //if err != nil {
//   //    log.Println(err.Error())
//   //}
//   //dir := filepath.Dir(path)
//   //log.Println(dir)
//   tableStruct:= NewTableStruct()
//   err := tableStruct.Table(`product`).EnableJsonTag(true).Dsn(`root:123456@(127.0.0.1:3306)/chatr?charset=utf8`).Run()
//   log.Println(`*****:`,err)
//}

type TableStruct struct {
	dsn            string
	savePath       string
	db             *xorm.Engine
	table          string
	prefix         string
	config         *Config
	err            error
	realNameMethod string
	enableJsonTag  bool // 是否添加json的tag, 默认不添加
	//packageName    string // 生成struct的包名(默认为空的话, 则取名为: package model)
	//tagKey         string // tag字段的key值,默认是orm
	dateToTime bool // 是否将 date相关字段转换为 time.Time,默认否
}

type Config struct {
	StructNameToHump bool // 结构体名称是否转为驼峰式，默认为false
	RmTagIfUcFirsted bool // 如果字段首字母本来就是大写, 就不添加tag, 默认false添加, true不添加
	TagToLower       bool // tag的字段名字是否转换为小写, 如果本身有大写字母的话, 默认false不转
	JsonTagToHump    bool // json tag是否转为驼峰，默认为false，不转换
	UcFirstOnly      bool // 字段首字母大写的同时, 是否要把其他字母转换为小写,默认false不转换
	SeperatFile      bool // 每个struct放入单独的文件,默认false,放入同一个文件
}

func NewTableStruct() *TableStruct {
	return &TableStruct{}
}

func (t *TableStruct) Dsn(d string) *TableStruct {
	t.dsn = d
	return t
}

func (t *TableStruct) DB(d *xorm.Engine) *TableStruct {
	t.db = d
	return t
}
func (t *TableStruct) SavePath(p string) *TableStruct {
	t.savePath = p
	return t
}
func (t *TableStruct) Table(tab string) *TableStruct {
	t.table = tab
	return t
}

func (t *TableStruct) EnableJsonTag(p bool) *TableStruct {
	t.enableJsonTag = p
	return t
}

func (t *TableStruct) DateToTime(d bool) *TableStruct {
	t.dateToTime = d
	return t
}

func (t *TableStruct) Config(c *Config) *TableStruct {
	t.config = c
	return t
}

func (t *TableStruct) Run() error {
	if t.config == nil {
		t.config = new(Config)
	}
	t.dialMysql()

	if t.err != nil {
		return t.err
	}

	tableColumns, err := t.getColumns()
	if err != nil {
		return err
	}
	// 组装struct
	var structContent string
	structContent += "type " + toStr(t.table) + " struct {\n"
	depth := 1
	for _, v := range tableColumns {
		// 字段注释
		var clumnComment, str string
		if !v.IsNullAble {
			str += ` not null`
		}
		if v.ColumnKey != `` {
			str += ` pk unique`
		}
		if v.Extra != `` {
			str += ` ` + v.Extra
		}
		str += ` ` + strings.ToUpper(v.ColumnType)
		Tag := fmt.Sprintf("`json:\"%s\" xorm:\"%s\"`", toStr1(v.ColumnName), str)
		if v.ColumnComment != "" {
			clumnComment = fmt.Sprintf(" // %s", v.ColumnComment)
		}
		structContent += fmt.Sprintf("%s%s %s %s%s\n",
			tab(depth), v.ColumnName, v.DataType, Tag, clumnComment)

	}
	// 添加 method 获取真实表名
	if t.realNameMethod != "" {
		structContent += fmt.Sprintf("func (%s) %s() string {\n",
			t.table, t.realNameMethod)
		structContent += fmt.Sprintf("%sreturn \"%s\"\n",
			tab(depth), t.table)
		structContent += "}\n\n"
	}
	structContent += tab(depth-1) + "}\n\n"
	var savePath = t.savePath
	if t.savePath == `` {
		savePath = "model.go"
	}
	// 如果有引入 time.Time, 则需要引入 time 包
	var importContent string
	if strings.Contains(structContent, "time.Time") {
		importContent = "import \"time\"\n\n"
	}
	// 包名
	var packageName = "package model\n\n"


	filePath := fmt.Sprintf("%s", savePath)
	f, err := os.Create(filePath)
	if err != nil {
		log.Println("Can not write file")
		return err
	}
	defer f.Close()
	f.WriteString(packageName + importContent + structContent)

	cmd := exec.Command("gofmt", "-w", filePath)
	cmd.Run()
	return nil
}
func tab(depth int) string {
	return strings.Repeat("\t", depth)
}

func (t *TableStruct) dialMysql() {
	if t.db == nil {
		if t.dsn == "" {
			t.err = errors.New("dsn数据库配置缺失")
			return
		}
		//t.db, t.err = sql.Open("mysql", t.dsn)
		t.db, t.err = xorm.NewEngine("mysql", t.dsn)
		if t.err != nil {
			log.Println(t.err)
		}
	}
	return
}

type column struct {
	TableSchema     string //数据库名
	TableName       string //表名
	ColumnName      string //字段名称
	OrdinalPosition int    //顺序
	IsNullAble      bool   //是否为空
	DataType        string //字段类型
	ColumnType      string //varchar(3072)
	CharacterMaxim  int    //字段大小
	ColumnComment   string //字段说明
	ColumnKey       string //主键
	Extra           string
}

func (t *TableStruct) getColumns(table ...string) (tableColumns []column, err error) {
	if t.table == `` {
		log.Println(`table 不能为空`)
		err = errors.New(`table.empty`)
		return
	}

	tableColumns = make([]column, 0)
	sqlStr := `SELECT * FROM information_schema.COLUMNS 
		WHERE table_schema = DATABASE() AND TABLE_NAME='` + t.table + `'`
	sqlStr += " order by TABLE_NAME asc, ORDINAL_POSITION asc"
	rows, err := t.db.QueryString(sqlStr)
	if err != nil {
		log.Println("getColumns:", err)
		return
	}

	for _, v := range rows {
		obj := column{}
		if value, ok := v[`TABLE_SCHEMA`]; ok {
			obj.TableSchema = value
		}
		if value, ok := v[`TABLE_NAME`]; ok {
			obj.TableName = value
		}
		if value, ok := v[`COLUMN_NAME`]; ok {

			obj.ColumnName = toStr(value)
		}
		if value, ok := v[`ORDINAL_POSITION`]; ok {
			obj.OrdinalPosition, _ = strconv.Atoi(value)
		}
		if value, ok := v[`IS_NULLABLE`]; ok {
			if value == `YES` {
				obj.IsNullAble = true
			}
		}
		if value, ok := v[`DATA_TYPE`]; ok {
			if _, ok := typeForMysqlToGo[value]; ok {
				obj.DataType = typeForMysqlToGo[value]
			}
		}
		if value, ok := v[`COLUMN_TYPE`]; ok {
			obj.ColumnType = value
		}
		if value, ok := v[`TABLE_SCHEMA`]; ok {
			obj.TableSchema = value
		}
		if value, ok := v[`CHARACTER_MAXIMUM_LENGTH`]; ok {
			if len(value) > 0 {
				obj.CharacterMaxim, _ = strconv.Atoi(value)
			}

		}
		if value, ok := v[`COLUMN_COMMENT`]; ok {
			obj.ColumnComment = value
		}
		if value, ok := v[`COLUMN_KEY`]; ok {
			obj.ColumnKey = value
		}
		if value, ok := v[`EXTRA`]; ok {
			obj.Extra = value
		}
		tableColumns = append(tableColumns, obj)
	}

	return
}

func toStr(str string) string {
	split := strings.Split(str, `_`)
	resp := ``
	for _, v := range split {

		if len(v) >= 1 {
			resp += strings.ToUpper(v[0:1]) + v[1:]
		}
	}
	return resp
}

func toStr1(str string) string {
	split := strings.Split(str, `_`)
	resp := ``
	for index, v := range split {
		if index == 0 {
			resp += v
			continue
		}
		if len(v) >= 1 {
			resp += strings.ToUpper(v[0:1]) + v[1:]
		}
	}
	return resp
}
